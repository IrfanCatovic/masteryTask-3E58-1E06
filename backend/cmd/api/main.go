package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"masterytask/internal/config"
	"masterytask/internal/db"
	"masterytask/internal/document"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration from config.go
	cfg := config.Load()

	// Connect to the database
	gormDB, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}

	log.Println("DB connection OK")


	if err := gormDB.AutoMigrate(&document.Document{}); err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}

	// Start the server
	router := gin.Default()
	// Check server health
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	// Check database health
	router.GET("/db-health", func(c *gin.Context) {
		sqlDB, err := gormDB.DB()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":   "error",
				"database": "unavailable",
				"message":  "failed to access sql db handle",
			})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := sqlDB.PingContext(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":   "error",
				"database": "disconnected",
				"message":  err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":   "ok",
			"database": "connected",
		})
	})

	// Create one hardcoded document to quickly verify DB insert flow.
	router.POST("/documents/mock", func(c *gin.Context) {
		doc := document.Document{
			DocumentType:   "invoice",
			SupplierName:   "Acme d.o.o.",
			DocumentNumber: "INV-MOCK-001",
			Status:         "uploaded",
		}

		if err := gormDB.Create(&doc).Error; err != nil { 
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to insert mock document",
				"error":   err.Error(),
			})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"status":   "ok",
			"message":  "mock document created",
			"document": doc,
		})
	})

	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}