package main

import (
	"fmt"
	"log"
	"net/http"
	"tg-movie-bot/config"
	"tg-movie-bot/internal/bot"
	"tg-movie-bot/internal/repository"
	"tg-movie-bot/pkg/telegram"

	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configurations: %v", err)
	}

	log.Printf("[INFO] System is working in %s mode...", cfg.Env)

	pgRepo, err := repository.NewPostgresRepository(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[CRITICAL] Postgres Error: %v", err)
	}
	defer pgRepo.Pool.Close()

	redisRepo, err := repository.NewRedisRepository(cfg.RedisURL)
	if err != nil {
		log.Fatalf("[CRITICAL] Redis error: %v", err)
	}
	defer func(Client *redis.Client) {
		err := Client.Close()
		if err != nil {
			log.Fatalf("[CRITICAL] Redis closure error: %v", err)
		}
	}(redisRepo.Client)

	tgClient := telegram.NewTelegramClient(cfg.TelegramBotToken)

	botService := bot.NewBotService(cfg, pgRepo, redisRepo, tgClient)

	botHandler := bot.NewBotHandler(botService)

	err = bot.LoadLocales("locales")
	if err != nil {
		log.Fatalf("[CRITICAL] Failed to load locales: %v", err)
	}
	http.HandleFunc("/webhook", botHandler.WebhookHTTPHandler)

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := fmt.Fprintf(w, "Bot Engine is working fine!")
		if err != nil {
			return
		}
	})

	log.Printf("[START] Webhook Server started on port: %s ...", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, nil); err != nil {
		log.Fatalf("Error running server: %v", err)
	}
}
