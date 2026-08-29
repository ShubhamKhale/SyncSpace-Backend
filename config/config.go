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
	GinMode   string // gin.ReleaseMode / gin.DebugMode; defaults to debug
	DBURL     string
	JWTSecret string
	RedisURL  string // redis://:password@host:port/db — optional; disables WebSocket if empty

	// Email (Brevo) — all optional; email sending is skipped when BrevoAPIKey is empty.
	BrevoAPIKey    string
	BrevoFromEmail string
	BrevoFromName  string
	FrontendURL    string // base URL used to construct invite links

	// Cloudinary — all optional; avatar upload is disabled when CloudinaryCloudName is empty.
	CloudinaryCloudName string
	CloudinaryAPIKey    string
	CloudinaryAPISecret string
}

// Load reads the .env file and returns a populated Config.
// Missing required fields are logged as fatal errors.
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("[CONFIG] No .env file found, reading from environment")
	}

	cfg := &Config{
		Port:      getEnv("PORT", "8080"),
		GinMode:   getEnv("GIN_MODE", "debug"),
		DBURL:     getEnv("DB_URL", ""),
		JWTSecret: getEnv("JWT_SECRET", ""),
		RedisURL:  getEnv("REDIS_URL", ""),

		BrevoAPIKey:    getEnv("BREVO_API_KEY", ""),
		BrevoFromEmail: getEnv("BREVO_FROM_EMAIL", "noreply@syncspace.app"),
		BrevoFromName:  getEnv("BREVO_FROM_NAME", "SyncSpace"),
		FrontendURL:    getEnv("FRONTEND_URL", "http://localhost:3000"),

		CloudinaryCloudName: getEnv("CLOUDINARY_CLOUD_NAME", ""),
		CloudinaryAPIKey:    getEnv("CLOUDINARY_API_KEY", ""),
		CloudinaryAPISecret: getEnv("CLOUDINARY_API_SECRET", ""),
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
