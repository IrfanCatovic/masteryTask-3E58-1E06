package document

import (
	"encoding/csv"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func registerUploadRoutes(router *gin.Engine, gormDB *gorm.DB) {
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

		doc, parseErr := parseUploadedDocument(fileHeader)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "failed to parse uploaded file",
				"error":   parseErr.Error(),
			})
			return
		}

		issues := ValidateDocument(doc)
		dupIssues, err := issuesForDuplicateDocumentNumber(gormDB, doc.DocumentNumber)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to check duplicate document number",
				"error":   err.Error(),
			})
			return
		}
		issues = append(issues, dupIssues...)
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

		if err := tx.Create(&doc).Error; err != nil {
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

func parseUploadedDocument(fileHeader *multipart.FileHeader) (Document, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return Document{}, err
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	switch ext {
	case ".csv":
		return parseCSVDocument(file)
	case ".txt":
		return parseTXTDocument(file)
	default:
		return Document{}, errors.New("unsupported file type; use .csv or .txt")
	}
}

func parseCSVDocument(r io.Reader) (Document, error) {
	reader := csv.NewReader(r)
	rows, err := reader.ReadAll()
	if err != nil {
		return Document{}, err
	}
	if len(rows) < 2 {
		return Document{}, errors.New("csv must contain header and at least one data row")
	}

	headers := make(map[string]int)
	for i, h := range rows[0] {
		headers[strings.ToLower(strings.TrimSpace(h))] = i //ovde pretvaramo header u lowercase i brisemo razmake
		//headers je map[string]int, gde key je header a value je index
		//i je index kolone, h je header string
	}

	required := []string{"document_type", "supplier_name", "document_number"}
	for _, key := range required {//ovde proveravamo da li imamo sve obavezne kolone
		if _, ok := headers[key]; !ok {
			return Document{}, errors.New("csv missing required column: " + key)//ako ne nađemo kolonu, vraćamo error
		}
	}

	first := rows[1] //prva vrsta je headeri, pa prva vrsta je prva kolona
	get := func(name string) string {//ovo get je funkcija koja prima ime kolone i vraća vrednost kolone
		//ovde npr primim "document_type", pa tražim index kolone "document_type" u headers mapi i to je 0
		idx, ok := headers[name]//ovde tražimo index kolone u headers mapi
		//idx je index kolone, ok je bool koji označava da li smo našli kolonu
		if !ok || idx >= len(first) {
			return ""//ako ne nađemo kolonu ili je index veći od dužine prve vrste, vraćamo prazan string
		}
		return strings.TrimSpace(first[idx])//ovde vraćamo vrednost kolone
	}

	doc := Document{
		DocumentType:   get("document_type"),
		SupplierName:   get("supplier_name"),
		DocumentNumber: get("document_number"),
		Status:         strings.TrimSpace(get("status")),
		Currency:       strings.TrimSpace(getOptional(headers, first, "currency")),
		Subtotal:       parseFloat(getOptional(headers, first, "subtotal")),
		TaxRate:        parseFloat(getOptional(headers, first, "tax_rate")),
		DiscountRate:   parseFloat(getOptional(headers, first, "discount_rate")),
		Total:          parseFloat(getOptional(headers, first, "total")),
	}
	if doc.Status == "" {
		doc.Status = "uploaded"
	}

	// Optional line-item columns from each row.
	//ovde proveravamo da li imamo kolone description, quantity, unit_price, line_total
	_, hasDesc := headers["description"]
	_, hasQty := headers["quantity"]
	_, hasUnit := headers["unit_price"]
	_, hasTotal := headers["line_total"]
	if hasDesc && hasQty && hasUnit && hasTotal {//ako imamo sve kolone, prolazimo kroz sve vrste
		for _, row := range rows[1:] { //radimo for loop kroz sve vrste osim prve vrste (headeri)
			item := LineItem{ 
				Description: readRowValue(row, headers["description"]),//ovde vraćamo vrednost kolone description
				Quantity:    parseFloat(readRowValue(row, headers["quantity"])),
				UnitPrice:   parseFloat(readRowValue(row, headers["unit_price"])),
				LineTotal:   parseFloat(readRowValue(row, headers["line_total"])),
			}
			if item.Description != "" {
				doc.LineItems = append(doc.LineItems, item)
			}
		}
	}

	return doc, nil
}

func parseTXTDocument(r io.Reader) (Document, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return Document{}, err
	}

	doc := Document{Status: "uploaded"}
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
		}
	}

	if doc.DocumentType == "" && doc.DocumentNumber == "" {
		return Document{}, errors.New("txt must include at least invoice type and document number")
	}

	return doc, nil
}

func getOptional(headers map[string]int, row []string, name string) string {
	idx, ok := headers[name]
	if !ok || idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
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
