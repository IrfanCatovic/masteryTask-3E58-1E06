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
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ledongthuc/pdf"
	"gorm.io/gorm"
)

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
		return parsePDFDocument(file)
	case ".png", ".jpg", ".jpeg", ".webp":
		return parseImageDocument(ctx, file, fileHeader.Filename, opts)
	default:
		return parseResult{}, errors.New("unsupported file type; use .csv, .txt, .pdf, or a supported image (.png, .jpg, .jpeg, .webp)")
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

// parsePDFDocument extracts plain text with github.com/ledongthuc/pdf and reuses the TXT parser.
// Image-only (scanned) PDFs often yield no text layer upload still succeeds with a validation issue.
func parsePDFDocument(r io.Reader) (parseResult, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return parseResult{}, err
	}
	pdfReader, err := pdf.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return parseResult{}, fmt.Errorf("invalid or encrypted pdf: %w", err)
	}
	textReader, err := pdfReader.GetPlainText()
	if err != nil {
		return parseResult{}, fmt.Errorf("pdf text extraction failed: %w", err)
	}
	rawText, err := io.ReadAll(textReader)
	if err != nil {
		return parseResult{}, err
	}
	text := strings.TrimSpace(string(rawText))

	res, err := parseTXTDocument(strings.NewReader(text))
	if err != nil {
		return parseResult{}, err
	}
	if text == "" {
		res.ParseIssues = append(res.ParseIssues, ValidationIssue{
			Code:      "PDF_NO_EXTRACTABLE_TEXT",
			Message:   "no extractable text in this pdf (image-only scans need OCR)",
			Severity:  IssueSeverityError,
			Resolved:  false,
		})
	}
	return res, nil
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
		Currency:       strings.ToUpper(get("currency")),
		Subtotal:       parseFloat(get("subtotal")),
		TaxRate:        parseFloat(get("tax_rate")),
		DiscountRate:   parseFloat(get("discount_rate")),
		Total:          parseFloat(get("total")),
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

	// Optional line-item columns from each row.
	_, hasDesc := headers["description"]
	_, hasQty := headers["quantity"]
	_, hasUnit := headers["unit_price"]
	_, hasTotal := headers["line_total"]
	if hasDesc && hasQty && hasUnit && hasTotal && len(rows) >= 2 {
		for _, row := range rows[1:] {
			item := LineItem{
				Description: readRowValue(row, headers["description"]),
				Quantity:    parseFloat(readRowValue(row, headers["quantity"])),
				UnitPrice:   parseFloat(readRowValue(row, headers["unit_price"])),
				LineTotal:   parseFloat(readRowValue(row, headers["line_total"])),
			}
			if item.Description != "" {
				doc.LineItems = append(doc.LineItems, item)
			}
		}
	}

	return parseResult{Document: doc, ParseIssues: parseIssues}, nil
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
	case "desc", "description", "item", "name":
		headers["description"] = colIdx
	case "qty", "quantity", "q":
		headers["quantity"] = colIdx
	case "price", "unit_price", "unitprice", "rate", "unit":
		headers["unit_price"] = colIdx
	case "line_total", "line total", "amount", "amt":
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

	doc := Document{Status: "uploaded"}
	parseIssues := make([]ValidationIssue, 0)
	lines := make([]string, 0)
	for _, raw := range strings.Split(string(b), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}

	// Real-world TXT format from the task dataset:
	// Line 1: "Invoice TXT-1"
	// Line 2: "Total: 406 EUR"
	if len(lines) > 0 {
		titleParts := strings.Fields(lines[0])
		if len(titleParts) >= 2 {
			doc.DocumentType = strings.ToLower(strings.TrimSpace(titleParts[0]))
			doc.DocumentNumber = strings.TrimSpace(titleParts[1])
		}
	}
	if len(lines) > 1 {
		second := lines[1]
		if strings.HasPrefix(strings.ToLower(second), "total:") {
			totalPayload := strings.TrimSpace(strings.TrimPrefix(second, "Total:"))
			totalParts := strings.Fields(totalPayload)
			if len(totalParts) >= 1 {
				doc.Total = parseFloat(totalParts[0])
			}
			if len(totalParts) >= 2 {
				doc.Currency = strings.ToUpper(strings.TrimSpace(totalParts[1]))
			}
		}
	}

	// Backward-compatible fallback for key:value txt format.
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		switch key {
		case "document_type":
			doc.DocumentType = val
		case "supplier_name":
			doc.SupplierName = val
		case "document_number":
			doc.DocumentNumber = val
		case "status":
			if val != "" {
				doc.Status = val
			}
		case "currency":
			doc.Currency = strings.ToUpper(strings.TrimSpace(val))
		case "subtotal":
			doc.Subtotal = parseFloat(val)
		case "tax_rate":
			doc.TaxRate = parseFloat(val)
		case "discount_rate":
			doc.DiscountRate = parseFloat(val)
		case "total":
			doc.Total = parseFloat(val)
		case "issue_date":
			if val != "" {
				t, err := parseYYYYMMDDOptional(val)
				if err != nil {
					parseIssues = append(parseIssues, newInvalidDateIssue("issue_date", val))
				} else {
					doc.IssueDate = t
				}
			}
		case "due_date":
			if val != "" {
				t, err := parseYYYYMMDDOptional(val)
				if err != nil {
					parseIssues = append(parseIssues, newInvalidDateIssue("due_date", val))
				} else {
					doc.DueDate = t
				}
			}
		}
	}

	if len(doc.LineItems) == 0 {
		doc.LineItems = parseLineItemsTableFromLines(lines)
	}

	return parseResult{Document: doc, ParseIssues: parseIssues}, nil
}

// parseLineItemsTableFromLines finds a header row (desc/qty/price/total or equivalents) and
// parses following rows into LineItems. Helps OCR/PDF text and plain-text invoice dumps.
func parseLineItemsTableFromLines(lines []string) []LineItem {
	var out []LineItem
	for i := 0; i < len(lines); i++ {
		cells := splitTableRow(lines[i])
		if len(cells) < 4 {
			continue
		}
		h := make(map[string]int)
		for j, c := range cells {
			key := strings.ToLower(strings.TrimSpace(c))
			registerLineItemColumnAliases(h, key, j)
		}
		if _, ok := h["description"]; !ok {
			continue
		}
		if _, ok := h["quantity"]; !ok {
			continue
		}
		if _, ok := h["unit_price"]; !ok {
			continue
		}
		if _, ok := h["line_total"]; !ok {
			continue
		}
		maxIdx := h["description"]
		for _, k := range []string{"quantity", "unit_price", "line_total"} {
			if h[k] > maxIdx {
				maxIdx = h[k]
			}
		}
		for j := i + 1; j < len(lines); j++ {
			row := splitTableRow(lines[j])
			if len(row) <= maxIdx {
				break
			}
			item := LineItem{
				Description: readRowValue(row, h["description"]),
				Quantity:    parseFloat(readRowValue(row, h["quantity"])),
				UnitPrice:   parseFloat(readRowValue(row, h["unit_price"])),
				LineTotal:   parseFloat(readRowValue(row, h["line_total"])),
			}
			if strings.TrimSpace(item.Description) == "" {
				continue
			}
			out = append(out, item)
		}
		break
	}
	return out
}

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
	r := csv.NewReader(strings.NewReader(line))
	r.FieldsPerRecord = -1
	if rec, err := r.Read(); err == nil && len(rec) > 1 {
		return rec
	}
	return strings.Fields(line)
}

func readRowValue(row []string, idx int) string {//dobijamo vrednost kolone iz reda
	if idx < 0 || idx >= len(row) {//ako je index negativan ili veći od dužine reda, vraćamo prazan string
		return ""
	}
	return strings.TrimSpace(row[idx])//ovde vraćamo vrednost kolone
}

func parseFloat(v string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)//ovde pretvaramo string u float64
	return f//ovde vraćamo float64
}
