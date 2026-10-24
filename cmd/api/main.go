package main

import (
	"log"
	"net/http"

	"github.com/ariefusry/diecast-auction/internal/config"
	"github.com/ariefusry/diecast-auction/internal/db"
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

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	log.Printf("api listening on :%s", cfg.Port)
	// ponytail: plain ListenAndServe, graceful shutdown lands in Phase 5.
	log.Fatal(http.ListenAndServe(":"+cfg.Port, mux))
}
