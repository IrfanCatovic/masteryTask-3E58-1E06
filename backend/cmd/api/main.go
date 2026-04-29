package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"masterytask/internal/config"
	"masterytask/internal/db"

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

	// Start the server
	router := gin.Default()
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})
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

	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}