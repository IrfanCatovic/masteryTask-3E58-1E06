package document

import (
	"net/http"
	"strings"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type createDocumentRequest struct {
	DocumentType   string `json:"document_type"`
	SupplierName   string `json:"supplier_name"`
	DocumentNumber string `json:"document_number"`
	Status         string `json:"status"`
}

// RegisterRoutes wires document-related HTTP routes.
func RegisterRoutes(router *gin.Engine, gormDB *gorm.DB) {
	// Create a document from JSON payload.
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

		req.DocumentType = strings.TrimSpace(req.DocumentType)
		req.SupplierName = strings.TrimSpace(req.SupplierName)
		req.DocumentNumber = strings.TrimSpace(req.DocumentNumber)
		req.Status = strings.TrimSpace(req.Status)

		if req.DocumentType == "" || req.SupplierName == "" || req.DocumentNumber == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "document_type, supplier_name, and document_number are required",
			})
			return
		}

		if req.Status == "" {
			req.Status = "uploaded"
		}

		doc := Document{
			DocumentType:   req.DocumentType,
			SupplierName:   req.SupplierName,
			DocumentNumber: req.DocumentNumber,
			Status:         req.Status,
		}

		if err := gormDB.Create(&doc).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to create document",
				"error":   err.Error(),
			})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"status":   "ok",
			"message":  "document created",
			"document": doc,
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

		c.JSON(http.StatusOK, gin.H{
			"status":   "ok",
			"document": doc,
		})
	})
}
