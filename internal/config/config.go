// Package config loads runtime settings from environment variables (12-Factor).
package config

import (
	"errors"
	"os"
)

type Config struct {
	DatabaseURL string
	Port        string
	JWTSecret   []byte
	// GeminiAPIKey is optional: without it AI search falls back to keyword search.
	GeminiAPIKey string
	GeminiModel  string
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
	if cfg.GeminiModel == "" {
		cfg.GeminiModel = "gemini-3.8-flash"
	}
	return cfg, nil
}
