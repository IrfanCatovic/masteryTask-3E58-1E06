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

var allowedStatuses = map[string]struct{}{
	"uploaded":     {},
	"needs_review": {},
	"validated":    {},
	"rejected":     {},
}


func RegisterRoutes(router *gin.Engine, gormDB *gorm.DB) {
	registerUploadRoutes(router, gormDB)

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

		if err := tx.Create(&doc).Error; err != nil {
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
		if err := gormDB.First(&doc, uint(id)).Error; err != nil {
			//ovde nadjemo document po id-u
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
	if strings.TrimSpace(documentNumber) == "" {
		return nil, nil
	}
	var n int64
	if err := db.Model(&Document{}).
		Where("document_number = ?", documentNumber).
		Count(&n).Error; err != nil {
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
