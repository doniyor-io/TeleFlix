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
			language_code VARCHAR(10) DEFAULT 'uz',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS language_code VARCHAR(10) DEFAULT 'uz';`,
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

	log.Println("[INFO] Postgres tables are checked and ready")
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

func (r *PostgresRepository) SaveUserLang(ctx context.Context, userID int64, username, lang string) error {
	query := `INSERT INTO users (id, username, language_code)
			  VALUES ($1, $2, $3)
              ON CONFLICT (id) DO UPDATE SET username = $2, language_code = $3
	`

	_, err := r.Pool.Exec(ctx, query, userID, username, lang)
	return err
}

func (r *PostgresRepository) GetUserLang(ctx context.Context, userID int64) (string, error) {
	query := `SELECT language_code FROM users WHERE id = $1`
	var lang string
	err := r.Pool.QueryRow(ctx, query, userID).Scan(&lang)
	if err != nil {
		return "uz", nil
	}
	return lang, nil
}

func (r *PostgresRepository) GetTotalUsersCount(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM users`
	var count int
	err := r.Pool.QueryRow(ctx, query).Scan(&count)
	return count, err
}

func (r *PostgresRepository) GetTotalMoviesCount(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM movies`
	var count int
	err := r.Pool.QueryRow(ctx, query).Scan(&count)
	return count, err
}

func (r *PostgresRepository) AddChannel(ctx context.Context, tgChannelID int64, inviteLink string) error {
	query := `INSERT INTO channels (tg_channel_id, invite_link, is_active) 
			  VALUES ($1, $2, true) 
			  ON CONFLICT (tg_channel_id) DO UPDATE SET invite_link = $2, is_active = true`
	_, err := r.Pool.Exec(ctx, query, tgChannelID, inviteLink)
	return err
}

func (r *PostgresRepository) DeleteChannel(ctx context.Context, tgChannelID int64) error {
	query := `DELETE FROM channels WHERE tg_channel_id = $1`
	_, err := r.Pool.Exec(ctx, query, tgChannelID)
	return err
}

func (r *PostgresRepository) GetAllMovies(ctx context.Context) ([]map[string]interface{}, error) {
	query := `SELECT id, instagram_url, tg_file_id, caption FROM movies ORDER BY created_at DESC`
	rows, err := r.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var movies []map[string]interface{}
	for rows.Next() {
		var id int
		var instaURL, fileID, caption string
		if err := rows.Scan(&id, &instaURL, &fileID, &caption); err != nil {
			return nil, err
		}
		movies = append(movies, map[string]interface{}{
			"id":               id,
			"instagram_url":    instaURL,
			"telegram_file_id": fileID,
			"caption":          caption,
		})
	}
	return movies, nil
}
