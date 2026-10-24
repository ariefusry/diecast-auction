// Package config loads runtime settings from environment variables (12-Factor).
package config

import (
	"errors"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL string
	Port        string
	JWTSecret   []byte
	// GeminiAPIKey is optional: without it AI search falls back to keyword search.
	GeminiAPIKey string
	GeminiModel  string
	// CORSOrigins are the browser origins allowed to call the API.
	CORSOrigins []string
	// UploadDir stores item photos (mount a volume there in production).
	UploadDir string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		Port:         os.Getenv("PORT"),
		JWTSecret:    []byte(os.Getenv("JWT_SECRET")),
		GeminiAPIKey: os.Getenv("GEMINI_API_KEY"),
		GeminiModel:  os.Getenv("GEMINI_MODEL"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("config: DATABASE_URL is required")
	}
	// 32 bytes = HS256 key size; shorter secrets are brute-forceable.
	if len(cfg.JWTSecret) < 32 {
		return Config{}, errors.New("config: JWT_SECRET must be at least 32 bytes")
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	for _, o := range strings.Split(os.Getenv("CORS_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			cfg.CORSOrigins = append(cfg.CORSOrigins, o)
		}
	}
	if cfg.CORSOrigins == nil {
		cfg.CORSOrigins = []string{"http://localhost:5173"} // Vite dev server
	}
	if cfg.UploadDir = os.Getenv("UPLOAD_DIR"); cfg.UploadDir == "" {
		cfg.UploadDir = "uploads"
	}
	if cfg.GeminiModel == "" {
		cfg.GeminiModel = "gemini-3.5-flash-lite"
	}
	return cfg, nil
}
