package config

import (
	"strings"
	"testing"
)

var validSecret = strings.Repeat("s", 32)

func setValidEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@db:5432/x")
	t.Setenv("JWT_SECRET", validSecret)
	t.Setenv("PORT", "")
}

func TestLoadReadsEnv(t *testing.T) {
	setValidEnv(t)
	t.Setenv("PORT", "9090")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DatabaseURL != "postgres://u:p@db:5432/x" || cfg.Port != "9090" || string(cfg.JWTSecret) != validSecret {
		t.Fatalf("got %+v", cfg)
	}
}

func TestLoadDefaultsPort(t *testing.T) {
	setValidEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "8080" {
		t.Fatalf("Port = %q, want 8080", cfg.Port)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	setValidEnv(t)
	t.Setenv("DATABASE_URL", "")

	if _, err := Load(); err == nil {
		t.Fatal("want error when DATABASE_URL is empty")
	}
}

func TestLoadRejectsShortJWTSecret(t *testing.T) {
	setValidEnv(t)
	t.Setenv("JWT_SECRET", strings.Repeat("s", 31))

	if _, err := Load(); err == nil {
		t.Fatal("want error when JWT_SECRET is shorter than 32 bytes")
	}
}
