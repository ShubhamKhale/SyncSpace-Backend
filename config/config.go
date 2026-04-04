// Package config loads application configuration from the environment.
// It reads a .env file (if present) and exposes a Config struct used
// throughout the application.
package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration for the application.
type Config struct {
	Port      string
	DBURL     string
	JWTSecret string
}

// Load reads the .env file and returns a populated Config.
// Missing required fields are logged as fatal errors.
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("[CONFIG] No .env file found, reading from environment")
	}

	cfg := &Config{
		Port:      getEnv("PORT", "8080"),
		DBURL:     getEnv("DB_URL", ""),
		JWTSecret: getEnv("JWT_SECRET", ""),
	}

	if cfg.DBURL == "" {
		log.Fatal("[CONFIG] DB_URL is required")
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
