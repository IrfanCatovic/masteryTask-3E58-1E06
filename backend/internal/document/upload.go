package document

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ledongthuc/pdf"
	"gorm.io/gorm"
)

// multiSpaceRe matches two or more whitespace characters (used to split aligned table rows from OCR/PDF text).
var multiSpaceRe = regexp.MustCompile(`[ \t]{2,}`)

// invoiceNumberRe pulls the document number from labels like "Invoice No. INV-1234" or "INVOICE #143999".
// The "no/number/#" marker is required so a bare "INVOICE" line doesn't get mistaken for the number itself.
var invoiceNumberRe = regexp.MustCompile(`(?i)(?:invoice|document|order|receipt|po|bill)\s*(?:no\.?|number|num\.?|#)\s*[:#]?\s*([A-Za-z0-9][A-Za-z0-9\-_/]{0,40})`)

// inlineLabelRe finds an embedded "Label:" inside an already-running line — used to split rows like
// "From: ACME  Number: INV-1  Date: 2024-01-01" that PDF/OCR text sometimes flattens into one line.
// The label is one alphabetic token (no spaces) so we don't accidentally swallow values; splits are
// only honoured when the prefix already carries its own colon (see splitInlineLabels), which keeps
// compound labels like "Total Due" intact.
var inlineLabelRe = regexp.MustCompile(`\s([A-Za-z][A-Za-z\-_]{1,30}):\s`)

// titleTypeKeywords are the only first-tokens that may promote a colon-less first line into
// "<type> <number>". This prevents table headers like "Description Qty Unit Price" from being
// misread as the document title.
var titleTypeKeywords = map[string]struct{}{
	"invoice":   {},
	"receipt":   {},
	"bill":      {},
	"order":     {},
	"purchase":  {},
	"po":        {},
	"quote":     {},
	"quotation": {},
	"faktura":   {},
	"racun":     {},
	"račun":     {},
}

// UploadOptions configures optional behaviour for multipart ingestion (e.g. OCR.space for images).
type UploadOptions struct {
	OCRSpaceAPIKey string
}

func registerUploadRoutes(router *gin.Engine, gormDB *gorm.DB, uploadOpts UploadOptions) {
	// Upload and ingest a CSV or TXT document file.
	router.POST("/documents/upload", func(c *gin.Context) {
		fileHeader, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "file is required in multipart field 'file'",
			})
			return
		}

		parsed, parseErr := parseUploadedDocument(c.Request.Context(), fileHeader, uploadOpts)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "failed to parse uploaded file",
				"error":   parseErr.Error(),
			})
			return
		}
		doc := parsed.Document

		dupIssues, err := issuesForDuplicateDocumentNumber(gormDB, doc.DocumentNumber)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to check duplicate document number",
				"error":   err.Error(),
			})
			return
		}
		if len(dupIssues) > 0 {
			c.JSON(http.StatusConflict, gin.H{
				"status":  "error",
				"code":    "DUPLICATE_DOCUMENT_NUMBER",
				"message": "a document with this number already exists",
			})
			return
		}

		issues := ValidateDocument(doc)
		issues = append(issues, parsed.ParseIssues...)
		if len(issues) > 0 {
			doc.Status = "needs_review"
		}

		tx := gormDB.Begin()
		if tx.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to start database transaction",
				"error":   tx.Error.Error(),
			})
			return
		}

		// Do not let GORM persist LineItems here — we set DocumentID and save them in a second step.
		if err := tx.Omit("LineItems").Create(&doc).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to create document",
				"error":   err.Error(),
			})
			return
		}

		if len(doc.LineItems) > 0 {
			for i := range doc.LineItems {
				doc.LineItems[i].DocumentID = doc.ID
			}
			if err := tx.Create(&doc.LineItems).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":  "error",
					"message": "failed to save line items",
					"error":   err.Error(),
				})
				return
			}
		}

		if len(issues) > 0 {
			for i := range issues {
				issues[i].DocumentID = doc.ID
			}
			if err := tx.Create(&issues).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":  "error",
					"message": "failed to save validation issues",
					"error":   err.Error(),
				})
				return
			}
		}

		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to commit database transaction",
				"error":   err.Error(),
			})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"status":       "ok",
			"message":      "document uploaded and processed",
			"document":     doc,
			"issues_count": len(issues),
			"issues":       issues,
		})
	})
}

func parseUploadedDocument(ctx context.Context, fileHeader *multipart.FileHeader, opts UploadOptions) (parseResult, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return parseResult{}, err
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	switch ext {
	case ".csv":
		return parseCSVDocument(file)
	case ".txt":
		return parseTXTDocument(file)
	case ".pdf":
		return parsePDFDocument(ctx, file, fileHeader.Filename, opts)
	default:
		return parseResult{}, errors.New("unsupported file type; use .csv, .txt, or .pdf")
	}
}

// parseImageDocument runs OCR via OCR.space then reuses the TXT document parser.
func parseImageDocument(ctx context.Context, r io.Reader, filename string, opts UploadOptions) (parseResult, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return parseResult{}, err
	}
	text, err := OCRSpaceExtractText(ctx, opts.OCRSpaceAPIKey, b, filename)
	if err != nil {
		return parseResult{}, err
	}
	text = strings.TrimSpace(text)

	res, err := parseTXTDocument(strings.NewReader(text))
	if err != nil {
		return parseResult{}, err
	}
	if text == "" {
		res.ParseIssues = append(res.ParseIssues, ValidationIssue{
			Code:      IssueCodeImageOCREmpty,
			Message:   "no text recognized in image (try clearer scan or different lighting)",
			Severity:  IssueSeverityError,
			Resolved:  false,
		})
	}
	return res, nil
}

// parsePDFDocument tries page-by-page text extraction first (preserves visual order better than the
// library's GetPlainText), falls back to GetPlainText, and finally to OCR.space when no usable text
// can be found and an OCR API key is configured. Always reuses the TXT parser for downstream parsing.
func parsePDFDocument(ctx context.Context, r io.Reader, filename string, opts UploadOptions) (parseResult, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return parseResult{}, err
	}
	pdfBytes := normalizePDFData(b)
	text, err := extractPDFText(pdfBytes)
	if err != nil {
		// Some providers prepend junk bytes before "%PDF-". If extraction still fails and OCR is
		// configured, try OCR as a resilience path instead of hard-failing the upload.
		if strings.TrimSpace(opts.OCRSpaceAPIKey) != "" {
			if ocrText, ocrErr := OCRSpaceExtractText(ctx, opts.OCRSpaceAPIKey, b, filename); ocrErr == nil {
				ocrText = strings.TrimSpace(ocrText)
				res, parseErr := parseTXTDocument(strings.NewReader(ocrText))
				if parseErr != nil {
					return parseResult{}, parseErr
				}
				if ocrText == "" {
					res.ParseIssues = append(res.ParseIssues, ValidationIssue{
						Code:     "PDF_NO_EXTRACTABLE_TEXT",
						Message:  "no extractable text in this pdf (image-only scans need OCR with a configured OCR_SPACE_API_KEY)",
						Severity: IssueSeverityError,
						Resolved: false,
					})
				}
				return res, nil
			}
		}
		return parseResult{}, fmt.Errorf("invalid or encrypted pdf: %w", err)
	}
	text = strings.TrimSpace(text)

	usedOCR := false
	if text == "" && strings.TrimSpace(opts.OCRSpaceAPIKey) != "" {
		if ocrText, ocrErr := OCRSpaceExtractText(ctx, opts.OCRSpaceAPIKey, b, filename); ocrErr == nil {
			text = strings.TrimSpace(ocrText)
			usedOCR = true
		}
	}

	res, err := parseTXTDocument(strings.NewReader(text))
	if err != nil {
		return parseResult{}, err
	}
	if text == "" {
		res.ParseIssues = append(res.ParseIssues, ValidationIssue{
			Code:     "PDF_NO_EXTRACTABLE_TEXT",
			Message:  "no extractable text in this pdf (image-only scans need OCR with a configured OCR_SPACE_API_KEY)",
			Severity: IssueSeverityError,
			Resolved: false,
		})
	} else if len(res.Document.LineItems) == 0 {
		// Surface a short peek of the extracted text so the reviewer can tell at a glance whether the
		// table layout was lost, the PDF was image-only, etc., without having to dig in the logs.
		preview := text
		if len(preview) > 240 {
			preview = preview[:240] + "…"
		}
		source := "pdf-text"
		if usedOCR {
			source = "ocr.space"
		}
		res.ParseIssues = append(res.ParseIssues, ValidationIssue{
			Code:     "PDF_NO_LINE_ITEMS",
			Message:  fmt.Sprintf("no line-item table detected (source: %s); raw preview: %s", source, preview),
			Severity: IssueSeverityWarning,
			Resolved: false,
		})
	}
	return res, nil
}

// extractPDFText prefers page-by-page row-based extraction (preserves table layout) and falls back
// to the library's whole-document GetPlainText when row extraction yields nothing.
func extractPDFText(data []byte) (string, error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	totalPages := reader.NumPage()
	for i := 1; i <= totalPages; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		rows, rowsErr := page.GetTextByRow()
		if rowsErr != nil {
			continue
		}
		for _, row := range rows {
			parts := make([]string, 0, len(row.Content))
			for _, t := range row.Content {
				if s := strings.TrimSpace(t.S); s != "" {
					parts = append(parts, s)
				}
			}
			if len(parts) == 0 {
				continue
			}
			// Two spaces preserve column boundaries for splitTableRow downstream.
			buf.WriteString(strings.Join(parts, "  "))
			buf.WriteString("\n")
		}
		buf.WriteString("\n")
	}
	if strings.TrimSpace(buf.String()) != "" {
		return buf.String(), nil
	}
	textReader, err := reader.GetPlainText()
	if err != nil {
		return "", nil
	}
	raw, err := io.ReadAll(textReader)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// normalizePDFData trims leading junk bytes when "%PDF-" appears shortly after the file start.
// This keeps parsing tolerant for PDFs produced by buggy exporters/proxies.
func normalizePDFData(data []byte) []byte {
	const maxProbe = 1024
	if len(data) == 0 {
		return data
	}
	if bytes.HasPrefix(data, []byte("%PDF-")) {
		return data
	}
	probe := data
	if len(probe) > maxProbe {
		probe = probe[:maxProbe]
	}
	if idx := bytes.Index(probe, []byte("%PDF-")); idx > 0 {
		return data[idx:]
	}
	return data
}

// parseResult vraća parsirani dokument zajedno sa "soft" issue-ima
// koje smo otkrili u toku parsiranja (npr. neparsabilan datum).
// Tako podržavamo "imperfect data" iz README-a umesto da odbacujemo upload.
type parseResult struct {
	Document   Document
	ParseIssues []ValidationIssue
}

func parseCSVDocument(r io.Reader) (parseResult, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return parseResult{}, err
	}
	if len(rows) < 1 {
		return parseResult{}, errors.New("csv is empty")
	}

	headers := buildCSVHeaderIndex(rows[0])

	var first []string
	if len(rows) >= 2 {
		first = rows[1]
	}

	// get reads a labelled column from the first data row only — fine for document-level fields, but
	// it is NEVER used for the line-item "total" column (which exists once per row). We sum the
	// per-row totals or read an explicit grand_total / invoice_total instead.
	get := func(name string) string {
		idx, ok := headers[name]
		if !ok || idx < 0 || idx >= len(first) {
			return ""
		}
		return strings.TrimSpace(first[idx])
	}

	doc := Document{
		DocumentType:   get("document_type"),
		SupplierName:   get("supplier_name"),
		DocumentNumber: get("document_number"),
		Status:         get("status"),
		Subtotal:       parseFloat(get("subtotal")),
		TaxRate:        parseFloat(get("tax_rate")),
		DiscountRate:   parseFloat(get("discount_rate")),
	}
	if cur := strings.TrimSpace(get("currency")); cur != "" {
		if hint := detectCurrency(cur); hint != "" {
			doc.Currency = hint
		} else {
			doc.Currency = strings.ToUpper(cur)
		}
	}
	if doc.Status == "" {
		doc.Status = "uploaded"
	}

	parseIssues := make([]ValidationIssue, 0)

	if s := get("issue_date"); s != "" {
		t, err := parseYYYYMMDDOptional(s)
		if err != nil {
			parseIssues = append(parseIssues, newInvalidDateIssue("issue_date", s))
		} else {
			doc.IssueDate = t
		}
	}
	if s := get("due_date"); s != "" {
		t, err := parseYYYYMMDDOptional(s)
		if err != nil {
			parseIssues = append(parseIssues, newInvalidDateIssue("due_date", s))
		} else {
			doc.DueDate = t
		}
	}

	// Line items come from the same CSV via the universal table detector — alias-first, positional
	// fallback. This handles both well-named exports (description/quantity/unit_price/line_total)
	// and short forms (desc/qty/price/total) without bespoke if/else.
	if len(rows) >= 2 {
		body := rows[1:]
		if items := lineItemsFromCSVRows(rows[0], body); len(items) > 0 {
			doc.LineItems = items
		}
	}

	// Document-level total: explicit grand_total/invoice_total/document_total wins; otherwise sum the
	// line totals when we have any. Do not fall back to the first row's "total" cell — that's a per-line value.
	if v := strings.TrimSpace(get("grand_total")); v != "" {
		doc.Total = parseFloat(v)
	} else if v := strings.TrimSpace(get("invoice_total")); v != "" {
		doc.Total = parseFloat(v)
	} else if v := strings.TrimSpace(get("document_total")); v != "" {
		doc.Total = parseFloat(v)
	} else if len(doc.LineItems) > 0 {
		var sum float64
		for _, it := range doc.LineItems {
			sum += it.LineTotal
		}
		doc.Total = sum
	}

	return parseResult{Document: doc, ParseIssues: parseIssues}, nil
}

// lineItemsFromCSVRows turns the CSV body rows into strings compatible with the same table
// detection used for OCR/PDF text. Reusing one detector keeps every format on a single rule set.
func lineItemsFromCSVRows(header []string, body [][]string) []LineItem {
	if items := lineItemsByAliasHeader(header, csvBodyToLines(header, body)); len(items) > 0 {
		return items
	}
	return lineItemsByPositionalHeader(header, csvBodyToLines(header, body))
}

func csvBodyToLines(header []string, body [][]string) []string {
	out := make([]string, 0, len(body))
	for _, row := range body {
		if len(row) == 0 {
			continue
		}
		// pad/truncate to header width so column indices line up after re-splitting
		if len(row) < len(header) {
			padded := make([]string, len(header))
			copy(padded, row)
			row = padded
		}
		// Re-emit as CSV so splitTableRow can parse with the same comma path.
		buf := &strings.Builder{}
		w := csv.NewWriter(buf)
		_ = w.Write(row[:len(header)])
		w.Flush()
		line := strings.TrimRight(buf.String(), "\n\r")
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

// buildCSVHeaderIndex maps the header row to column indices, including aliases for line-item
// exports that use short names (desc, qty, price, total) instead of description, quantity, etc.
func buildCSVHeaderIndex(headerRow []string) map[string]int {
	headers := make(map[string]int)
	for i, h := range headerRow {
		key := strings.ToLower(strings.TrimSpace(h))
		registerLineItemColumnAliases(headers, key, i)
	}
	return headers
}

// registerLineItemColumnAliases wires canonical keys expected by parseCSVDocument line-item logic.
func registerLineItemColumnAliases(headers map[string]int, rawLower string, colIdx int) {
	if rawLower == "" {
		return
	}
	headers[rawLower] = colIdx
	switch rawLower {
	case "desc", "description", "item", "name", "product", "service", "category":
		headers["description"] = colIdx
	case "qty", "quantity", "q", "count":
		headers["quantity"] = colIdx
	case "price", "unit_price", "unit price", "unitprice", "rate", "unit":
		headers["unit_price"] = colIdx
	case "line_total", "line total", "linetotal", "amount", "amt":
		headers["line_total"] = colIdx
	case "total":
		headers["total"] = colIdx
		if _, ok := headers["line_total"]; !ok {
			headers["line_total"] = colIdx
		}
	}
}

func parseTXTDocument(r io.Reader) (parseResult, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return parseResult{}, err
	}
	rawText := string(b)
	doc := Document{Status: "uploaded"}
	parseIssues := make([]ValidationIssue, 0)
	lines := splitNonEmptyLines(rawText)

	// First non-empty line acts as a "Title Number" header (e.g. "Invoice TXT-1") only when its
	// first token is a recognised document-type keyword. This stops table headers like
	// "Description Qty Unit Price Total" from polluting type/number.
	if len(lines) > 0 && !strings.Contains(lines[0], ":") {
		if titleParts := strings.Fields(lines[0]); len(titleParts) >= 2 {
			first := strings.ToLower(titleParts[0])
			if _, ok := titleTypeKeywords[first]; ok {
				doc.DocumentType = first
				doc.DocumentNumber = strings.TrimSpace(titleParts[1])
			}
		}
	}

	for _, line := range lines {
		for _, segment := range splitInlineLabels(line) {
			applyKeyValueLine(segment, &doc, &parseIssues)
		}
	}

	// Heuristic: if document number is still missing, look for "Invoice No. X" / "INVOICE #X" anywhere.
	if strings.TrimSpace(doc.DocumentNumber) == "" {
		if m := invoiceNumberRe.FindStringSubmatch(rawText); len(m) > 1 {
			doc.DocumentNumber = strings.TrimSpace(m[1])
		}
	}
	// Heuristic: infer document type from common keywords when missing.
	if strings.TrimSpace(doc.DocumentType) == "" {
		doc.DocumentType = inferDocumentType(rawText)
	}

	if len(doc.LineItems) == 0 {
		doc.LineItems = parseLineItemsTableFromLines(lines)
	}
	// When we have line items and the document didn't declare its own total via a labelled line,
	// fall back to summing line totals so doc.Total matches the table.
	if len(doc.LineItems) > 0 && doc.Total == 0 {
		var sum float64
		for _, it := range doc.LineItems {
			sum += it.LineTotal
		}
		doc.Total = sum
	}

	return parseResult{Document: doc, ParseIssues: parseIssues}, nil
}

// splitInlineLabels breaks a flattened line that contains multiple "Label: value" pairs into one
// segment per pair. Many PDF/OCR pipelines lose newlines between fields, leaving us with strings
// like "From: ACME  Number: INV-1  Date: 2024-01-01" that the colon-based parser would otherwise
// dump entirely into the first label's value. The regex requires a whitespace boundary before the
// label so timestamps like "12:30 PM" inside a value stay intact.
func splitInlineLabels(line string) []string {
	if !strings.Contains(line, ":") {
		return []string{line}
	}
	idxs := inlineLabelRe.FindAllStringIndex(line, -1)
	if len(idxs) == 0 {
		return []string{line}
	}
	parts := make([]string, 0, len(idxs)+1)
	prev := 0
	for _, idx := range idxs {
		// idx[0] sits on the whitespace before the label; we cut just after it so the label
		// starts the next segment.
		split := idx[0] + 1
		// Only honour the cut when the prefix already carries its own colon. Without that,
		// what we matched is just the second word of a compound label like "Total Due" — splitting
		// there would leave both halves orphaned.
		if !strings.Contains(line[prev:split], ":") {
			continue
		}
		if seg := strings.TrimSpace(line[prev:split]); seg != "" {
			parts = append(parts, seg)
		}
		prev = split
	}
	if tail := strings.TrimSpace(line[prev:]); tail != "" {
		parts = append(parts, tail)
	}
	if len(parts) == 0 {
		return []string{line}
	}
	return parts
}

// splitNonEmptyLines normalises CRLF/CR endings and drops empty lines so downstream parsers can
// treat lines positionally without worrying about layout artefacts.
func splitNonEmptyLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	out := make([]string, 0)
	for _, raw := range strings.Split(s, "\n") {
		if line := strings.TrimSpace(raw); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// applyKeyValueLine recognises "Label: value" lines and updates the document accordingly. Labels are
// normalised (lowercased, spaces -> underscores) so "Total Due" and "total_due" are equivalent.
func applyKeyValueLine(line string, doc *Document, parseIssues *[]ValidationIssue) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return
	}
	key := strings.ToLower(strings.TrimSpace(parts[0]))
	key = strings.ReplaceAll(key, " ", "_")
	val := strings.TrimSpace(parts[1])
	switch key {
	case "document_type", "type", "kind":
		if val != "" {
			doc.DocumentType = strings.ToLower(val)
		}
	case "supplier_name", "supplier", "vendor", "from", "seller":
		if val != "" {
			doc.SupplierName = val
		}
	case "document_number", "invoice_number", "invoice_no", "invoice_no.", "invoice_#",
		"number", "no", "no.", "ref", "reference":
		if val != "" {
			doc.DocumentNumber = val
		}
	case "status":
		if val != "" {
			doc.Status = val
		}
	case "currency":
		if cur := detectCurrency(val); cur != "" {
			doc.Currency = cur
		} else if val != "" {
			doc.Currency = strings.ToUpper(val)
		}
	case "subtotal", "sub_total", "net":
		if n, ok := extractFloat(val); ok {
			doc.Subtotal = n
		}
	case "tax_rate", "vat_rate", "tax", "vat":
		if n, ok := extractFloat(val); ok {
			doc.TaxRate = n
		}
	case "discount_rate", "discount":
		if n, ok := extractFloat(val); ok {
			doc.DiscountRate = n
		}
	case "total", "total_due", "grand_total", "amount_due", "balance_due", "ukupno", "iznos":
		if n, ok := extractFloat(val); ok {
			doc.Total = n
		}
		if doc.Currency == "" {
			if cur := detectCurrency(val); cur != "" {
				doc.Currency = cur
			}
		}
	case "issue_date", "date", "invoice_date":
		if val != "" {
			if t, err := parseYYYYMMDDOptional(val); err == nil {
				doc.IssueDate = t
			} else {
				*parseIssues = append(*parseIssues, newInvalidDateIssue("issue_date", val))
			}
		}
	case "due_date", "due", "payment_due":
		if val != "" {
			if t, err := parseYYYYMMDDOptional(val); err == nil {
				doc.DueDate = t
			} else {
				*parseIssues = append(*parseIssues, newInvalidDateIssue("due_date", val))
			}
		}
	}
}

// inferDocumentType makes a best-effort guess from the raw text when no explicit field is present.
// Returning an empty string means "still unknown" so validation will flag it as missing.
func inferDocumentType(text string) string {
	upper := strings.ToUpper(text)
	switch {
	case strings.Contains(upper, "INVOICE"), strings.Contains(upper, "FAKTURA"):
		return "invoice"
	case strings.Contains(upper, "RECEIPT"), strings.Contains(upper, "RAČUN"), strings.Contains(upper, "RACUN"):
		return "receipt"
	case strings.Contains(upper, "PURCHASE ORDER"), strings.Contains(upper, "ORDER FORM"):
		return "purchase_order"
	}
	return ""
}

// parseLineItemsTableFromLines walks free-text lines, looks for the first plausible header row
// (4+ cells), and tries an alias-based mapping first; if that fails, falls back to a positional
// heuristic (one text column + three numeric columns). Empty result means no recognizable table.
func parseLineItemsTableFromLines(lines []string) []LineItem {
	var headerBased []LineItem
	for i := 0; i < len(lines); i++ {
		header := splitTableRow(lines[i])
		if len(header) < 4 {
			continue
		}
		body := lines[i+1:]
		if items := lineItemsByAliasHeader(header, body); len(items) > 0 {
			headerBased = items
			break
		}
		if items := lineItemsByPositionalHeader(header, body); len(items) > 0 {
			headerBased = items
			break
		}
	}
	// Many PDFs/OCRs flatten or jumble headers so no clean 4-column header survives — and even when
	// they do, the line we picked might really be the first body row in disguise. We always try the
	// header-less scanner and prefer it when it finds more rows than the header-based path.
	fallback := lineItemsByTrailingNumerics(lines)
	if len(fallback) > len(headerBased) {
		return fallback
	}
	return headerBased
}

// lineItemsByTrailingNumerics is the header-less fallback. A row qualifies when it ends in three
// numeric tokens AND has at least one leading text token; we treat the trailing trio as
// (quantity, unit_price, line_total) in that order. To stay robust against false positives we
// require that at least half of the candidates satisfy quantity * unit_price ≈ line_total.
func lineItemsByTrailingNumerics(lines []string) []LineItem {
	type candidate struct {
		desc                 string
		qty, unit, lineTotal float64
		productMatch         bool
	}
	candidates := make([]candidate, 0)
	for _, line := range lines {
		row := splitTableRow(line)
		if len(row) < 4 {
			continue
		}
		n := len(row)
		q, okQ := extractFloat(row[n-3])
		u, okU := extractFloat(row[n-2])
		t, okT := extractFloat(row[n-1])
		if !(okQ && okU && okT) {
			continue
		}
		var dp []string
		for j := 0; j < n-3; j++ {
			if s := strings.TrimSpace(row[j]); s != "" {
				dp = append(dp, s)
			}
		}
		desc := strings.TrimSpace(strings.Join(dp, " "))
		if desc == "" {
			continue
		}
		// Drop header-looking rows so "Description Qty Price Total" never enters as data.
		if isLikelyHeaderDescription(desc) {
			continue
		}
		candidates = append(candidates, candidate{
			desc:         desc,
			qty:          q,
			unit:         u,
			lineTotal:    t,
			productMatch: approxProduct(q, u, t),
		})
	}
	if len(candidates) == 0 {
		return nil
	}
	matches := 0
	for _, c := range candidates {
		if c.productMatch {
			matches++
		}
	}
	// Need at least half of the candidates to satisfy qty*unit≈total before we trust the heuristic.
	if matches*2 < len(candidates) {
		return nil
	}
	out := make([]LineItem, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, LineItem{
			Description: c.desc,
			Quantity:    c.qty,
			UnitPrice:   c.unit,
			LineTotal:   c.lineTotal,
		})
	}
	return out
}

// approxProduct returns true when q*u is within ~5 % of t (or both are zero). Used by the
// header-less line-item detector to filter rows whose number triplet doesn't multiply out.
func approxProduct(q, u, t float64) bool {
	if q == 0 || u == 0 {
		return t == 0
	}
	expected := q * u
	if expected == 0 {
		return t == 0
	}
	diff := expected - t
	if diff < 0 {
		diff = -diff
	}
	if diff < 0.01 {
		return true
	}
	scale := expected
	if t > scale {
		scale = t
	}
	if scale < 0 {
		scale = -scale
	}
	return diff/scale < 0.05
}

// isLikelyHeaderDescription flags rows whose description column is just header words like
// "DESCRIPTION" / "ITEM" / "CATEGORY" so the trailing-numerics detector skips them when the
// table header itself accidentally ends in three numbers (e.g. an "ID, Qty, Total" preamble).
func isLikelyHeaderDescription(desc string) bool {
	lower := strings.ToLower(desc)
	for _, kw := range []string{"description", "item", "product", "service", "category", "name"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// lineItemsByAliasHeader maps known column names (description/qty/price/total etc.) to canonical
// fields and reads following rows. Returns empty slice when the header lacks the four required roles.
func lineItemsByAliasHeader(header []string, body []string) []LineItem {
	h := make(map[string]int)
	for j, c := range header {
		key := strings.ToLower(strings.TrimSpace(c))
		registerLineItemColumnAliases(h, key, j)
	}
	desc, hasDesc := h["description"]
	qty, hasQty := h["quantity"]
	unit, hasUnit := h["unit_price"]
	total, hasTotal := h["line_total"]
	if !(hasDesc && hasQty && hasUnit && hasTotal) {
		return nil
	}
	maxIdx := desc
	for _, k := range []int{qty, unit, total} {
		if k > maxIdx {
			maxIdx = k
		}
	}
	out := make([]LineItem, 0)
	for _, line := range body {
		row := splitTableRow(line)
		if len(row) <= maxIdx {
			break
		}
		item := LineItem{
			Description: readRowValue(row, desc),
			Quantity:    parseFloat(readRowValue(row, qty)),
			UnitPrice:   parseFloat(readRowValue(row, unit)),
			LineTotal:   parseFloat(readRowValue(row, total)),
		}
		if strings.TrimSpace(item.Description) == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

// lineItemsByPositionalHeader detects 1 description column + 3 numeric columns by sampling the body
// rows. When a body row has the same shape as the header we honour header column names
// (rate→unit, price/amount→total). When the body row has MORE columns than the header (typical
// when PDF text extraction leaks the description into multiple tokens) we fall back to "everything
// before the trailing numeric trio is the description" so rows like "Flyer Design 300 3 900" still
// work even though they split into 5 fields against a 4-column header.
func lineItemsByPositionalHeader(header []string, body []string) []LineItem {
	cols := len(header)
	if cols < 4 || len(body) == 0 {
		return nil
	}
	qtyCol, unitCol, totalCol := -1, -1, -1
	for j, h := range header {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "qty", "quantity", "q", "count":
			qtyCol = j
		case "rate", "unit", "unit_price", "unit price", "unitprice":
			unitCol = j
		case "price", "amount", "amt", "total", "line_total", "line total", "linetotal":
			totalCol = j
		}
	}
	out := make([]LineItem, 0, len(body))
	productMatches, productChecks := 0, 0
	for _, line := range body {
		row := splitTableRow(line)
		if len(row) < 4 {
			continue
		}
		// Trailing numeric trio: required for any row to qualify as a line item.
		n := len(row)
		tailQ, okQ := extractFloat(row[n-3])
		tailU, okU := extractFloat(row[n-2])
		tailT, okT := extractFloat(row[n-1])
		if !(okQ && okU && okT) {
			continue
		}
		// Pick column meanings: header alias mapping when shapes match AND the header column
		// actually points at a numeric cell, otherwise the trailing trio (positional fallback).
		var q, u, t float64
		if len(row) == cols && qtyCol >= 0 && unitCol >= 0 && totalCol >= 0 {
			if v, ok := extractFloat(row[qtyCol]); ok {
				q = v
			} else {
				q = tailQ
			}
			if v, ok := extractFloat(row[unitCol]); ok {
				u = v
			} else {
				u = tailU
			}
			if v, ok := extractFloat(row[totalCol]); ok {
				t = v
			} else {
				t = tailT
			}
		} else {
			q, u, t = tailQ, tailU, tailT
		}
		// Description = leading non-empty tokens up to (but not including) the trailing numeric trio.
		// This survives "Flyer Design" (two tokens) collapsing into one description string.
		var dp []string
		for j := 0; j < n-3; j++ {
			if s := strings.TrimSpace(row[j]); s != "" {
				dp = append(dp, s)
			}
		}
		desc := strings.TrimSpace(strings.Join(dp, " "))
		if desc == "" {
			continue
		}
		if isLikelyHeaderDescription(desc) {
			continue
		}
		productChecks++
		if approxProduct(q, u, t) {
			productMatches++
		}
		out = append(out, LineItem{
			Description: desc,
			Quantity:    q,
			UnitPrice:   u,
			LineTotal:   t,
		})
	}
	// Sanity gate: when the table is genuinely a line-item table, q*u≈t holds for the majority of
	// rows. If almost no row passes, we very likely picked up an unrelated number triplet table.
	if productChecks > 0 && productMatches*2 < productChecks {
		return nil
	}
	return out
}

// splitTableRow tokenizes a single free-text row into columns. Order of attempts: tabs, CSV (commas),
// 2+ whitespace runs (typical of OCR/PDF aligned columns), then single-space fallback.
func splitTableRow(line string) []string {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	if strings.Contains(line, "\t") {
		parts := strings.Split(line, "\t")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts
	}
	if strings.Contains(line, ",") {
		r := csv.NewReader(strings.NewReader(line))
		r.FieldsPerRecord = -1
		if rec, err := r.Read(); err == nil && len(rec) > 1 {
			for i := range rec {
				rec[i] = strings.TrimSpace(rec[i])
			}
			return rec
		}
	}
	if multiSpaceRe.MatchString(line) {
		raw := multiSpaceRe.Split(line, -1)
		out := make([]string, 0, len(raw))
		for _, p := range raw {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		if len(out) > 1 {
			return out
		}
	}
	return strings.Fields(line)
}

func readRowValue(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

// parseFloat is a forgiving wrapper around extractFloat, returning 0 when no number is found.
// Callers that need to distinguish "missing" from "explicit zero" should use extractFloat directly.
func parseFloat(v string) float64 {
	n, _ := extractFloat(v)
	return n
}

// extractFloat pulls a numeric value from free-form text (currency markers, thousand separators,
// trailing units etc.). Returns ok=false when no digit is present so callers can avoid overwriting
// previously parsed values with a default zero.
func extractFloat(v string) (float64, bool) {
	s := strings.TrimSpace(v)
	if s == "" {
		return 0, false
	}
	var b strings.Builder
	hasDigit := false
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			hasDigit = true
		case r == '.' || r == ',' || r == '-':
			b.WriteRune(r)
		}
	}
	if !hasDigit {
		return 0, false
	}
	cleaned := b.String()
	hasDot := strings.Contains(cleaned, ".")
	hasComma := strings.Contains(cleaned, ",")
	switch {
	case hasDot && hasComma:
		// Use whichever separator appears LAST as the decimal point and drop the other entirely.
		if strings.LastIndex(cleaned, ",") > strings.LastIndex(cleaned, ".") {
			cleaned = strings.ReplaceAll(cleaned, ".", "")
			cleaned = strings.Replace(cleaned, ",", ".", 1)
			cleaned = strings.ReplaceAll(cleaned, ",", "")
		} else {
			cleaned = strings.ReplaceAll(cleaned, ",", "")
		}
	case hasComma && !hasDot:
		// Single comma is treated as a decimal separator (EU style); multiple commas drop them all.
		if strings.Count(cleaned, ",") == 1 {
			cleaned = strings.Replace(cleaned, ",", ".", 1)
		} else {
			cleaned = strings.ReplaceAll(cleaned, ",", "")
		}
	}
	f, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// detectCurrency returns an uppercase 3-letter currency hint when the input contains a known code or symbol.
// Empty result means the caller should not overwrite an already known currency.
func detectCurrency(v string) string {
	upper := strings.ToUpper(v)
	for _, c := range []string{"EUR", "USD", "GBP", "CHF", "RSD", "BAM", "HRK"} {
		if strings.Contains(upper, c) {
			return c
		}
	}
	switch {
	case strings.Contains(v, "€"):
		return "EUR"
	case strings.Contains(v, "$"):
		return "USD"
	case strings.Contains(v, "£"):
		return "GBP"
	}
	return ""
}
