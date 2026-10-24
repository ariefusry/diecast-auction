package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ariefusry/diecast-auction/internal/config"
	"github.com/ariefusry/diecast-auction/internal/db"
	"github.com/ariefusry/diecast-auction/internal/handler"
	"github.com/ariefusry/diecast-auction/internal/llm"
	"github.com/ariefusry/diecast-auction/internal/middleware"
	"github.com/ariefusry/diecast-auction/internal/service"
	"github.com/ariefusry/diecast-auction/internal/worker"
	"github.com/ariefusry/diecast-auction/migrations"
)

func main() {
	// JSON logs; slog.SetDefault also routes the standard log package through it.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Migrate(cfg.DatabaseURL, migrations.FS); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
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

	var parse handler.ParseFunc
	if cfg.GeminiAPIKey != "" {
		parse = llm.NewGemini(cfg.GeminiAPIKey, cfg.GeminiModel).ParseQuery
	} else {
		log.Print("GEMINI_API_KEY not set: AI search uses keyword search")
	}
	srv := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: middleware.Log(slog.Default())(middleware.CORS(cfg.CORSOrigins)(
			handler.New(service.NewAuth(pool, cfg.JWTSecret), service.NewWallets(pool), auctions, parse, cfg.UploadDir))),
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
