package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisRepository struct {
	Client *redis.Client
}

func NewRedisRepository(redisURL string) (*RedisRepository, error) {
	client := redis.NewClient(&redis.Options{
		Addr: redisURL,
	})

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisRepository{
		Client: client,
	}, nil
}

func (r *RedisRepository) SetUserLanguageCache(
	ctx context.Context,
	userID int64,
	lang string,
) error {

	key := fmt.Sprintf("user_lang:%d", userID)

	return r.Client.Set(
		ctx,
		key,
		lang,
		24*time.Hour,
	).Err()
}

func (r *RedisRepository) GetUserLanguageCache(
	ctx context.Context,
	userID int64,
) (string, error) {

	key := fmt.Sprintf("user_lang:%d", userID)

	return r.Client.Get(ctx, key).Result()
}

func (r *RedisRepository) SetSubscriptionCache(
	ctx context.Context,
	userID int64,
	isSubscribed bool,
) error {

	key := fmt.Sprintf("subscription:%d", userID)

	return r.Client.Set(
		ctx,
		key,
		isSubscribed,
		5*time.Minute,
	).Err()
}

func (r *RedisRepository) GetSubscriptionCache(
	ctx context.Context,
	userID int64,
) (bool, bool, error) {

	key := fmt.Sprintf("subscription:%d", userID)

	res, err := r.Client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return false, false, nil
		}
		return false, false, err
	}

	isSubbed := res == "1" || res == "true"
	return isSubbed, true, nil
}

func (r *RedisRepository) DeleteSubscriptionCache(
	ctx context.Context,
	userID int64,
) error {

	key := fmt.Sprintf("subscription:%d", userID)

	return r.Client.Del(ctx, key).Err()
}

func (r *RedisRepository) SetPendingMovieFileID(ctx context.Context, userID int64, fileID string) error {
	key := fmt.Sprintf("pending_movie_file:%d", userID)
	return r.Client.Set(ctx, key, fileID, 30*time.Minute).Err()
}

func (r *RedisRepository) GetPendingMovieFileID(ctx context.Context, userID int64) (string, error) {
	key := fmt.Sprintf("pending_movie_file:%d", userID)
	return r.Client.Get(ctx, key).Result()
}

func (r *RedisRepository) DeletePendingMovieFileID(ctx context.Context, userID int64) error {
	key := fmt.Sprintf("pending_movie_file:%d", userID)
	return r.Client.Del(ctx, key).Err()
}
