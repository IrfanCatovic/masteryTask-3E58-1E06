package document

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RegisterRoutes wires document-related HTTP routes.
func RegisterRoutes(router *gin.Engine, gormDB *gorm.DB) {
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
}
