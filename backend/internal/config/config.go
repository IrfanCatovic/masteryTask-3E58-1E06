package config


import (
	"log"
	"os"

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
		DBHost:     mustGetEnv("DB_HOST"),
		DBUser:     mustGetEnv("DB_USER"),
		DBPassword: mustGetEnv("DB_PASSWORD"),
		DBName:     mustGetEnv("DB_NAME"),
		DBPort:     mustGetEnv("DB_PORT"),
		DBSSLMode:  mustGetEnv("DB_SSLMODE"),
		DBTimeZone: mustGetEnv("DB_TIMEZONE"),
	}
}