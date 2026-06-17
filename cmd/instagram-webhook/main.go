package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"tg-movie-bot/internal/instagramwebhook"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)

	cfg, err := instagramwebhook.LoadConfig(".env")
	if err != nil {
		logger.Fatalf("[CRITICAL] configuration error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatalf("[CRITICAL] postgres pool error: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Fatalf("[CRITICAL] postgres ping error: %v", err)
	}

	store := instagramwebhook.NewStore(pool)
	if err := store.EnsureSchema(ctx); err != nil {
		logger.Fatalf("[CRITICAL] schema setup error: %v", err)
	}

	handler := instagramwebhook.NewHandler(cfg.MetaWebhookVerifyToken, store, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook/instagram", handler.InstagramWebhook)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Printf("[START] Instagram webhook server listening on port %s", cfg.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("[CRITICAL] server error: %v", err)
	}
}
