// Package config loads application configuration from the environment.
// It reads a .env file (if present) and exposes a Config struct used
// throughout the application.
package config

import (
	"log"
	"os"
	"strings"

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

	// Ollama — local model server for AI chat/summarization in dev; no external API key.
	OllamaURL   string
	OllamaModel string

	// Groq — cloud LLM used in prod (deployed backends can't reach a local Ollama).
	// When GroqAPIKey is set, it takes priority over Ollama.
	GroqAPIKey string
	GroqModel  string

	// Diagram generation model. GroqDiagramWebSearch is a server-side kill
	// switch for Groq's built-in browser_search tool (openai/gpt-oss-*); users
	// still opt in per request.
	GroqDiagramModel     string
	GroqDiagramWebSearch bool
	// GroqDiagramFallbacks are tried in order (no tools, JSON mode) when the
	// diagram model fails. Each Groq model has its own rate limit, so more
	// models = more headroom. Defaults to GroqModel then qwen/qwen3.8-27b.
	GroqDiagramFallbacks []string
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

		OllamaURL:   getEnv("OLLAMA_URL", "http://localhost:11434"),
		OllamaModel: getEnv("OLLAMA_MODEL", "llama3.2:3b"),

		GroqAPIKey: getEnv("GROQ_API_KEY", ""),
		GroqModel:  getEnv("GROQ_MODEL", "openai/gpt-oss-20b"),

		GroqDiagramModel:     getEnv("GROQ_DIAGRAM_MODEL", "openai/gpt-oss-120b"),
		GroqDiagramWebSearch: getEnv("GROQ_DIAGRAM_WEB_SEARCH", "true") == "true",
	}

	cfg.GroqDiagramFallbacks = splitList(getEnv("GROQ_DIAGRAM_FALLBACKS", cfg.GroqModel+",qwen/qwen3.8-27b"))

	if cfg.DBURL == "" {
		log.Fatal("[CONFIG] DB_URL is required")
	}

	return cfg
}

// splitList parses a comma-separated env value, dropping blanks.
func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
