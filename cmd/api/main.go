package main

import (
	"context"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ariefusry/diecast-auction/internal/config"
	"github.com/ariefusry/diecast-auction/internal/db"
	"github.com/ariefusry/diecast-auction/internal/handler"
	"github.com/ariefusry/diecast-auction/internal/service"
	"github.com/ariefusry/diecast-auction/migrations"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Migrate(cfg.DatabaseURL, migrations.FS); err != nil {
		log.Fatal(err)
	}
	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	api := handler.New(service.NewAuth(pool, cfg.JWTSecret), service.NewWallets(pool))

	log.Printf("api listening on :%s", cfg.Port)
	// ponytail: plain ListenAndServe, graceful shutdown lands in Phase 5.
	log.Fatal(http.ListenAndServe(":"+cfg.Port, api))
}
