package main

import (
	"log"

	"masterytask/internal/config"
	"masterytask/internal/db"
)

func main() {
	// Load configuration from config.go
	cfg := config.Load()

	// Connect to the database
	_, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}

	log.Println("DB connection OK")
}