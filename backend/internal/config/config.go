package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost     string
	DBUser     string
	DBPassword string
	DBName     string
	DBPort     string
	DBSSLMode  string
	DBTimeZone string
	// Optional: image uploads use OCR.space; leave empty to disable image ingestion only.
	OCRSpaceAPIKey string
	// CORSAllowedOrigins: non-empty enables CORS for those exact Origin values (e.g. Vite + prod UI).
	// Empty: no CORS middleware (same-origin or API behind a same-host reverse proxy).
	CORSAllowedOrigins []string
}

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("Missing required env var: %s", key)
	}
	return v
}

func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Printf("No .env loaded (continuing with OS env): %v", err)
	}

	return Config{
		DBHost:             mustGetEnv("DB_HOST"),
		DBUser:             mustGetEnv("DB_USER"),
		DBPassword:         mustGetEnv("DB_PASSWORD"),
		DBName:             mustGetEnv("DB_NAME"),
		DBPort:             mustGetEnv("DB_PORT"),
		DBSSLMode:          mustGetEnv("DB_SSLMODE"),
		DBTimeZone:         mustGetEnv("DB_TIMEZONE"),
		OCRSpaceAPIKey:     strings.TrimSpace(os.Getenv("OCR_SPACE_API_KEY")),
		CORSAllowedOrigins: splitCommaList(os.Getenv("CORS_ALLOWED_ORIGINS")),
	}
}

func splitCommaList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}