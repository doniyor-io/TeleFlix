package repository

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
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

func (r *RedisRepository) GetSubscriptionCache(ctx context.Context, userID int64) (bool, bool, error) {
	key := fmt.Sprintf("sub:%d", userID)
	val, err := r.Client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	parsed, parseErr := strconv.ParseBool(val)
	if parseErr != nil {
		return false, false, parseErr
	}
	return parsed, true, nil
}

func (r *RedisRepository) InvalidateSubscriptionCache(ctx context.Context, userID int64) error {
	key := fmt.Sprintf("sub:%d", userID)
	return r.Client.Del(ctx, key).Err()
}

func (r *RedisRepository) SetUserLangCache(ctx context.Context, userID int64, lang string) error {
	key := fmt.Sprintf("lang:%d", userID)
	return r.Client.Set(ctx, key, lang, 24*time.Hour).Err()
}

// GetUserLangCache Get User's Language code from Redis Cache
func (r *RedisRepository) GetUserLangCache(ctx context.Context, userID int64) (string, error) {
	key := fmt.Sprintf("lang:%d", userID)
	return r.Client.Get(ctx, key).Result()
}
