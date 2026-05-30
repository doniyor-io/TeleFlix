//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"tg-movie-bot/internal/repository"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/tg_movie_bot?sslmode=disable"
	}
	repo, err := repository.NewPostgresRepository(dbURL)
	if err != nil {
		fmt.Println("DB error:", err)
		return
	}
	chans, err := repo.GetActiveChannels(context.Background())
	fmt.Println("Active channels:", chans, err)
}
