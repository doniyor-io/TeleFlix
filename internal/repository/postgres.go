package repository

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	Pool *pgxpool.Pool
}

func NewPostgresRepository(databaseURL string) (*PostgresRepository, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create PostgreSQL Pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("error connecting to PostgreSQL (Ping): %w", err)
	}

	repo := &PostgresRepository{Pool: pool}

	if err := repo.migrate(ctx); err != nil {
		return nil, fmt.Errorf("migration error: %w", err)
	}

	return repo, nil
}

func (r *PostgresRepository) migrate(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id BIGINT PRIMARY KEY,
			username VARCHAR(100),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS movies (
			id SERIAL PRIMARY KEY,
			instagram_url TEXT UNIQUE NOT NULL,
			tg_file_id TEXT NOT NULL,
			caption TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS channels (
			id SERIAL PRIMARY KEY,
			tg_channel_id BIGINT UNIQUE NOT NULL,
			invite_link TEXT NOT NULL,
			is_active BOOLEAN DEFAULT TRUE
		);`,
	}

	for _, q := range queries {
		if _, err := r.Pool.Exec(ctx, q); err != nil {
			return err
		}
	}

	log.Println("[INFO] Postgres jadvallari tekshirildi va tayyor.")
	return nil
}

func (r *PostgresRepository) SaveMovie(ctx context.Context, instagramURL, tgFileID, caption string) error {
	query := `INSERT INTO movies (instagram_url, tg_file_id, caption) 
			  VALUES ($1, $2, $3) 
			  ON CONFLICT (instagram_url) 
			  DO UPDATE SET tg_file_id = $2, caption = $3`
	_, err := r.Pool.Exec(ctx, query, instagramURL, tgFileID, caption)
	return err
}

func (r *PostgresRepository) GetMovieByInstagramURL(ctx context.Context, instagramURL string) (string, string, error) {
	query := `SELECT tg_file_id, caption FROM movies WHERE instagram_url = $1`
	var fileID, caption string
	err := r.Pool.QueryRow(ctx, query, instagramURL).Scan(&fileID, &caption)
	return fileID, caption, err
}

func (r *PostgresRepository) GetActiveChannels(ctx context.Context) ([]map[string]interface{}, error) {
	query := `SELECT tg_channel_id, invite_link FROM channels WHERE is_active = true`
	rows, err := r.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []map[string]interface{}
	for rows.Next() {
		var tgID int64
		var link string
		if err := rows.Scan(&tgID, &link); err != nil {
			return nil, err
		}
		channels = append(channels, map[string]interface{}{
			"id":   tgID,
			"link": link,
		})
	}
	return channels, nil
}
