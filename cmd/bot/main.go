package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"tg-movie-bot/config"
	"tg-movie-bot/internal/bot"
	"tg-movie-bot/internal/repository"
	"tg-movie-bot/pkg/telegram"

	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.Load()
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

	botService.SyncAdminMenuButtons(context.Background())
	// -----------------------------------------------------------------
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", botHandler.WebhookHTTPHandler)

	mux.HandleFunc("/api/meta/reel", botHandler.MetaReelHandler)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Bot Engine is working fine!")
	})

	handleAdmin := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, botHandler.AdminAuthMiddleware(handler))
	}

	handleAdmin("/api/admin/stats", botHandler.GetStatsHandler)
	handleAdmin("/api/admin/channels", botHandler.ChannelsHandler)
	handleAdmin("/api/admin/channels/delete", botHandler.DeleteChannelHandler)
	handleAdmin("/api/admin/movies", botHandler.GetMoviesHandler)
	handleAdmin("/api/admin/movies/delete", botHandler.DeleteMovieHandler)
	handleAdmin("/api/admin/movies/link-reel", botHandler.LinkReelHandler)
	handleAdmin("/api/admin/movies/top", botHandler.TopMoviesHandler)
	handleAdmin("/api/admin/users", botHandler.UsersHandler)

	frontendURLStr := os.Getenv("FRONTEND_INTERNAL_URL")
	if frontendURLStr == "" {
		frontendPort := os.Getenv("FRONTEND_PORT")
		if frontendPort == "" {
			frontendPort = "3000"
		}
		frontendURLStr = fmt.Sprintf("http://host.docker.internal:%s", frontendPort)
	}

	frontendURL, _ := url.Parse(frontendURLStr)
	proxy := httputil.NewSingleHostReverseProxy(frontendURL)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})

	finalHandler := botHandler.CorsMiddleware(mux)

	log.Printf("[START] Webhook & Admin API Server started on port: %s ...", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, finalHandler); err != nil {
		log.Fatalf("Error running server: %v", err)
	}
}
