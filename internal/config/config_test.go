package config

import "testing"

func TestLoadReadsEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@db:5432/x")
	t.Setenv("PORT", "9090")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DatabaseURL != "postgres://u:p@db:5432/x" || cfg.Port != "9090" {
		t.Fatalf("got %+v", cfg)
	}
}

func TestLoadDefaultsPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@db:5432/x")
	t.Setenv("PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "8080" {
		t.Fatalf("Port = %q, want 8080", cfg.Port)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	if _, err := Load(); err == nil {
		t.Fatal("want error when DATABASE_URL is empty")
	}
}
