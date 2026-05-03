package document

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type createDocumentRequest struct {
	DocumentType   string `json:"document_type"`
	SupplierName   string `json:"supplier_name"`
	DocumentNumber string `json:"document_number"`
	IssueDate      string `json:"issue_date"`
	DueDate        string `json:"due_date"`
	Status         string `json:"status"`
	LineItems      []createLineItemRequest `json:"line_items"`
}

type createLineItemRequest struct {
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	LineTotal   float64 `json:"line_total"`
}

type updateStatusRequest struct {
	Status string `json:"status"`
}

// updateDocumentRequest carries optional fields for manual correction.
// Pointer fields let us tell apart "field omitted" vs "field set to empty".
type updateDocumentRequest struct {
	DocumentType   *string  `json:"document_type"`
	SupplierName   *string  `json:"supplier_name"`
	DocumentNumber *string  `json:"document_number"`
	IssueDate      *string  `json:"issue_date"`
	DueDate        *string  `json:"due_date"`
	Currency       *string  `json:"currency"`
	Subtotal       *float64 `json:"subtotal"`
	TaxRate        *float64 `json:"tax_rate"`
	DiscountRate   *float64 `json:"discount_rate"`
	Total          *float64 `json:"total"`
}

var allowedStatuses = map[string]struct{}{
	"uploaded":     {},
	"needs_review": {},
	"validated":    {},
	"rejected":     {},
}


func RegisterRoutes(router *gin.Engine, gormDB *gorm.DB, uploadOpts UploadOptions) {
	registerUploadRoutes(router, gormDB, uploadOpts)

	router.POST("/documents", func(c *gin.Context) {
		var req createDocumentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "invalid JSON payload",
				"error":   err.Error(),
			})
			return
		}
		//delete spaces from strings
		req.DocumentType = strings.TrimSpace(req.DocumentType)
		req.SupplierName = strings.TrimSpace(req.SupplierName)
		req.DocumentNumber = strings.TrimSpace(req.DocumentNumber)
		req.IssueDate = strings.TrimSpace(req.IssueDate)
		req.DueDate = strings.TrimSpace(req.DueDate)
		req.Status = strings.TrimSpace(req.Status)
		for i := range req.LineItems {
			req.LineItems[i].Description = strings.TrimSpace(req.LineItems[i].Description)
		} //ovde se samo stringovi ciste od razmaka u svim poljima sa stringovima
		//treba nam for jer jedan dokument ima slice od vise itema

		if req.Status == "" {
			req.Status = "uploaded"
		}
		if _, ok := allowedStatuses[req.Status]; !ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "status must be one of: uploaded, needs_review, validated, rejected",
			})
			return
		}

		issueDate, err := parseYYYYMMDDOptional(req.IssueDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "issue_date must be empty or YYYY-MM-DD",
				"error":   err.Error(),
			})
			return
		}
		dueDate, err := parseYYYYMMDDOptional(req.DueDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "due_date must be empty or YYYY-MM-DD",
				"error":   err.Error(),
			})
			return
		}

		doc := Document{
			DocumentType:   req.DocumentType,
			SupplierName:   req.SupplierName,
			DocumentNumber: req.DocumentNumber,
			IssueDate:      issueDate,
			DueDate:        dueDate,
			Status:         req.Status,
		}
		
		doc.LineItems = make([]LineItem, 0, len(req.LineItems))
		for _, item := range req.LineItems {
			doc.LineItems = append(doc.LineItems, LineItem{
				Description: item.Description,
				Quantity:    item.Quantity,
				UnitPrice:   item.UnitPrice,
				LineTotal:   item.LineTotal,
			})
		}
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
		if len(issues) > 0 {
			// If we found issues, document should move into review state.
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
		// Persist line items only after document has an ID.
		if len(doc.LineItems) > 0 {
			for i := range doc.LineItems {
				doc.LineItems[i].DocumentID = doc.ID //doc.ID npr 17, doc.LineItems[0].DocumentID = 17 i popuni za sve iteme u slice-u doc.LineItems
			}
			if err := tx.Create(&doc.LineItems).Error; err != nil {//ovde cuvamo line items u bazu
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":  "error",
					"message": "failed to save line items",
					"error":   err.Error(),
				})
				return
			}
		}
		//ovde cuvamo validation issues u bazu
		if len(issues) > 0 {
			for i := range issues {
				issues[i].DocumentID = doc.ID //doc.ID npr 17, issues[0].DocumentID = 17 i popuni za sve issue-e u slice-u issues

			}
			if err := tx.Create(&issues).Error; err != nil {//ovde cuvamo validation issues u bazu
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":  "error",
					"message": "failed to save validation issues",
					"error":   err.Error(),
				})
				return
			}
		}
		if err := tx.Commit().Error; err != nil {//ako prodje uspesno dodavanje dokumenta u bazu, 
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to commit database transaction",
				"error":   err.Error(),
			})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"status":       "ok",
			"message":      "document created",
			"document":     doc,
			"issues_count": len(issues),
			"issues":       issues,
		})
	})

	// List documents, optionally filtered by status (?status=uploaded).
	router.GET("/documents", func(c *gin.Context) {
		var docs []Document
		status := c.Query("status")

		query := gormDB
		if status != "" {
			query = query.Where("status = ?", status)
		}

		if err := query.Order("id desc").Find(&docs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to fetch documents",
				"error":   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"count":     len(docs),
			"documents": docs,
		})
	})

	// Fetch a single document by id (/documents/:id).
	router.GET("/documents/:id", func(c *gin.Context) {
		idParam := c.Param("id")
		id, err := strconv.ParseUint(idParam, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "invalid document id",
			})
			return
		}

		var doc Document
		if err := gormDB.
			Preload("LineItems").
			Preload("Issues").
			First(&doc, uint(id)).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"status":  "error",
					"message": "document not found",
				})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to fetch document",
				"error":   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":   "ok",
			"document": doc,
		})
	})

	// Update only document status (/documents/:id/status).
	router.PATCH("/documents/:id/status", func(c *gin.Context) {
		idParam := c.Param("id")
		id, err := strconv.ParseUint(idParam, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "invalid document id",
			})
			return
		}

		var req updateStatusRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "invalid JSON payload",
				"error":   err.Error(),
			})
			return
		}

		req.Status = strings.TrimSpace(req.Status)
		if _, ok := allowedStatuses[req.Status]; !ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "status must be one of: uploaded, needs_review, validated, rejected",
			})
			return
		}

		var doc Document
		if err := gormDB.Preload("LineItems").Preload("Issues").First(&doc, uint(id)).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"status":  "error",
					"message": "document not found",
				})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to fetch document",
				"error":   err.Error(),
			})
			return
		}

		// Guard: cannot mark a document "validated" while it still has missing required fields or
		// unresolved error-severity issues. The reviewer must fix the data first.
		if req.Status == "validated" {
			if blockers := blockersForValidation(doc); len(blockers) > 0 {
				c.JSON(http.StatusConflict, gin.H{
					"status":   "error",
					"code":     "VALIDATION_BLOCKED",
					"message":  "cannot validate while required fields are missing or unresolved error issues remain",
					"blockers": blockers,
				})
				return
			}
		}

		if err := gormDB.Model(&doc).Update("status", req.Status).Error; err != nil {
			//ovde azuriramo status u bazi
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to update status",
				"error":   err.Error(),
			})
			return
		}
		doc.Status = req.Status //ovde azuriramo status documenta u structu za odgovor

		c.JSON(http.StatusOK, gin.H{
			"status":   "ok",
			"message":  "document status updated",
			"document": doc,
		})
	})

	// Manual correction of extracted fields. Re-runs validation and
	// rewrites the issue list so the review interface always reflects
	// the latest state of the document.
	router.PATCH("/documents/:id", func(c *gin.Context) {
		idParam := c.Param("id")
		id, err := strconv.ParseUint(idParam, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "invalid document id",
			})
			return
		}

		var req updateDocumentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "invalid JSON payload",
				"error":   err.Error(),
			})
			return
		}

		var doc Document
		if err := gormDB.Preload("LineItems").First(&doc, uint(id)).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"status":  "error",
					"message": "document not found",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to fetch document",
				"error":   err.Error(),
			})
			return
		}

		// Track date issues that come from manual edits with bad input.
		dateIssues := make([]ValidationIssue, 0)

		if req.DocumentType != nil {
			doc.DocumentType = strings.TrimSpace(*req.DocumentType)
		}
		if req.SupplierName != nil {
			doc.SupplierName = strings.TrimSpace(*req.SupplierName)
		}
		if req.DocumentNumber != nil {
			doc.DocumentNumber = strings.TrimSpace(*req.DocumentNumber)
		}
		if req.Currency != nil {
			doc.Currency = strings.ToUpper(strings.TrimSpace(*req.Currency))
		}
		if req.Subtotal != nil {
			doc.Subtotal = *req.Subtotal
		}
		if req.TaxRate != nil {
			doc.TaxRate = *req.TaxRate
		}
		if req.DiscountRate != nil {
			doc.DiscountRate = *req.DiscountRate
		}
		if req.Total != nil {
			doc.Total = *req.Total
		}
		if req.IssueDate != nil {
			val := strings.TrimSpace(*req.IssueDate)
			if val == "" {
				doc.IssueDate = nil
			} else if t, err := parseYYYYMMDDOptional(val); err == nil {
				doc.IssueDate = t
			} else {
				dateIssues = append(dateIssues, newInvalidDateIssue("issue_date", val))
			}
		}
		if req.DueDate != nil {
			val := strings.TrimSpace(*req.DueDate)
			if val == "" {
				doc.DueDate = nil
			} else if t, err := parseYYYYMMDDOptional(val); err == nil {
				doc.DueDate = t
			} else {
				dateIssues = append(dateIssues, newInvalidDateIssue("due_date", val))
			}
		}

		// Recompute issues against the new state.
		issues := ValidateDocument(doc)
		issues = append(issues, dateIssues...)
		dupIssues, err := issuesForDuplicateDocumentNumberExcluding(gormDB, doc.DocumentNumber, doc.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to check duplicate document number",
				"error":   err.Error(),
			})
			return
		}
		issues = append(issues, dupIssues...)

		// Auto-flag status when new issues appear; leave manual statuses
		// (validated/rejected) untouched if user already triaged them.
		if len(issues) > 0 && doc.Status != "rejected" {
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

		if err := tx.Save(&doc).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to update document",
				"error":   err.Error(),
			})
			return
		}

		if err := tx.Where("document_id = ?", doc.ID).Delete(&ValidationIssue{}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to clear stale issues",
				"error":   err.Error(),
			})
			return
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

		// Reload with fresh associations for the response.
		var fresh Document
		if err := gormDB.Preload("LineItems").Preload("Issues").First(&fresh, doc.ID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to reload document",
				"error":   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":       "ok",
			"message":      "document updated",
			"document":     fresh,
			"issues_count": len(fresh.Issues),
		})
	})

	// Delete document and related line items / validation issues.
	router.DELETE("/documents/:id", func(c *gin.Context) {
		idParam := c.Param("id")
		id, err := strconv.ParseUint(idParam, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "invalid document id",
			})
			return
		}

		var doc Document
		if err := gormDB.First(&doc, uint(id)).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"status":  "error",
					"message": "document not found",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to fetch document",
				"error":   err.Error(),
			})
			return
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

		if err := tx.Where("document_id = ?", id).Delete(&ValidationIssue{}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to delete validation issues",
				"error":   err.Error(),
			})
			return
		}
		if err := tx.Where("document_id = ?", id).Delete(&LineItem{}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to delete line items",
				"error":   err.Error(),
			})
			return
		}
		if err := tx.Delete(&Document{}, uint(id)).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to delete document",
				"error":   err.Error(),
			})
			return
		}
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to commit database transaction",
				"error":   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"message": "document deleted",
			"id":      id,
		})
	})
}

// parseYYYYMMDDOptional accepts empty string or a calendar date "2006-01-02".
func parseYYYYMMDDOptional(s string) (*time.Time, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// issuesForDuplicateDocumentNumber returns a DUPLICATE_DOCUMENT_NUMBER issue when that number already exists.
func issuesForDuplicateDocumentNumber(db *gorm.DB, documentNumber string) ([]ValidationIssue, error) {
	return issuesForDuplicateDocumentNumberExcluding(db, documentNumber, 0)
}

// issuesForDuplicateDocumentNumberExcluding behaves like issuesForDuplicateDocumentNumber
// but ignores the document with id == excludeID, which is needed when revalidating
// an existing record after a manual edit (so it doesn't flag itself as a duplicate).
func issuesForDuplicateDocumentNumberExcluding(db *gorm.DB, documentNumber string, excludeID uint) ([]ValidationIssue, error) {
	if strings.TrimSpace(documentNumber) == "" {
		return nil, nil
	}
	var n int64
	q := db.Model(&Document{}).Where("document_number = ?", documentNumber)
	if excludeID != 0 {
		q = q.Where("id <> ?", excludeID)
	}
	if err := q.Count(&n).Error; err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	return []ValidationIssue{{
		Code:      "DUPLICATE_DOCUMENT_NUMBER",
		Message:   "document number already exists",
		Severity:  IssueSeverityError,
		FieldName: "document_number",
		Resolved:  false,
	}}, nil
}

// blockersForValidation returns the human-readable reasons that prevent a document from being moved
// to "validated" status: missing required fields and any unresolved error-severity issues. An empty
// slice means validation is allowed.
func blockersForValidation(doc Document) []string {
	blockers := make([]string, 0)
	if strings.TrimSpace(doc.DocumentType) == "" {
		blockers = append(blockers, "document_type is required")
	}
	if strings.TrimSpace(doc.SupplierName) == "" {
		blockers = append(blockers, "supplier_name is required")
	}
	if strings.TrimSpace(doc.DocumentNumber) == "" {
		blockers = append(blockers, "document_number is required")
	}
	for _, iss := range doc.Issues {
		if !iss.Resolved && iss.Severity == IssueSeverityError {
			blockers = append(blockers, "unresolved error: "+iss.Code)
		}
	}
	return blockers
}
