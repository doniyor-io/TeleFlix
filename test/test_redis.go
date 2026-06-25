//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"tg-movie-bot/internal/repository"
)

func main() {
	ctx := context.Background()
	repo, err := repository.NewRedisRepository("localhost:6379")
	if err != nil {
		fmt.Println(err)
		return
	}
	repo.SetSubscriptionCache(ctx, 123, false)
	val, err := repo.GetSubscriptionCache(ctx, 123)
	fmt.Println("Result of false cast:", val, err)
	repo.SetSubscriptionCache(ctx, 123, true)
	val, err = repo.GetSubscriptionCache(ctx, 123)
	fmt.Println("Result of true cast:", val, err)
}
