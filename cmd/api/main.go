package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ariefusry/diecast-auction/internal/config"
	"github.com/ariefusry/diecast-auction/internal/db"
	"github.com/ariefusry/diecast-auction/internal/handler"
	"github.com/ariefusry/diecast-auction/internal/service"
	"github.com/ariefusry/diecast-auction/internal/worker"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	auctions := service.NewAuctions(pool)
	workerDone := make(chan struct{})
	go func() {
		worker.Run(ctx, time.Second, auctions.CloseExpired)
		close(workerDone)
	}()

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler.New(service.NewAuth(pool, cfg.JWTSecret), service.NewWallets(pool), auctions),
	}
	go func() {
		log.Printf("api listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	log.Print("shutting down")
	// Stop accepting connections and let in-flight requests (and their transactions) finish.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	<-workerDone
}
