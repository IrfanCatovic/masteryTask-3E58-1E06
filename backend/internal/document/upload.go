package document

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func registerUploadRoutes(router *gin.Engine, _ *gorm.DB) {
	router.POST("/documents/upload", func(c *gin.Context) {
		file, err := c.FormFile("file") // sa ovim citaš fajl iz forme
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "file is required in multipart field 'file'",
			})
			return
		}

		ext := strings.ToLower(filepath.Ext(file.Filename)) //ovde dobiješ ekstenziju fajla
		if ext != ".csv" && ext != ".txt" { //ovde proveravaš da li je fajl csv ili txt, ako nije, vrati error
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "unsupported file type; only .csv and .txt are allowed",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":   "ok",
			"message":  "file accepted",
			"filename": file.Filename,
			"type":     ext,
		})
	})
}
