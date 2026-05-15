package repository

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisRepository struct {
	Client *redis.Client
}

func NewRedisRepository(redisURL string) (*RedisRepository, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := redis.NewClient(&redis.Options{
		Addr: redisURL,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("error connecting to redis: %w", err)
	}

	log.Println("[INFO] Connected to Redis...")
	return &RedisRepository{Client: client}, nil
}

func (r *RedisRepository) SetSubscriptionCache(ctx context.Context, userID int64, isSubbed bool) error {
	key := fmt.Sprintf("sub:%d", userID)
	return r.Client.Set(ctx, key, isSubbed, 5*time.Minute).Err()
}

func (r *RedisRepository) GetSubscriptionCache(ctx context.Context, userID int64) (bool, error) {
	key := fmt.Sprintf("sub:%d", userID)
	return r.Client.Get(ctx, key).Bool()
}

func (r *RedisRepository) InvalidateSubscriptionCache(ctx context.Context, userID int64) error {
	key := fmt.Sprintf("sub:%d", userID)
	return r.Client.Del(ctx, key).Err()
}
