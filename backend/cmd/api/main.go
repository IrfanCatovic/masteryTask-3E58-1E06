package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("Missing required env var: %s", key)
	}
	return v
}

func main() {

	if err := godotenv.Load(); err != nil {

		log.Printf("No .env loaded (continuing with OS env): %v", err)
	}

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		mustGetEnv("DB_HOST"),
		mustGetEnv("DB_USER"),
		mustGetEnv("DB_PASSWORD"),
		mustGetEnv("DB_NAME"),
		mustGetEnv("DB_PORT"),
		mustGetEnv("DB_SSLMODE"),
		mustGetEnv("DB_TIMEZONE"),
	)

	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to open DB connection: %v", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		log.Fatalf("Failed to get sql.DB from gorm: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		log.Fatalf("DB ping failed: %v", err)
	}

	log.Println("DB connection OK")
}