package repository

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"time"

	_ "github.com/jackc/pgx/v5"
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
			code VARCHAR(20) UNIQUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`ALTER TABLE movies ADD COLUMN IF NOT EXISTS code VARCHAR(20) UNIQUE;`,
		`CREATE TABLE IF NOT EXISTS channels (
			id SERIAL PRIMARY KEY,
			tg_channel_id BIGINT UNIQUE NOT NULL,
			invite_link TEXT NOT NULL,
			is_active BOOLEAN DEFAULT TRUE
		);`,
		`CREATE TABLE IF NOT EXISTS reels (
			id SERIAL PRIMARY KEY,
			reel_link TEXT UNIQUE NOT NULL,
			movie_id INT REFERENCES movies(id) ON DELETE SET NULL,
			linked_at TIMESTAMP DEFAULT NOW()
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

// GenerateUniqueMovieCode - Kolliya (To'qnashuv) tekshiruvi bilan ishlaydigan unique kod generatori
func (r *PostgresRepository) GenerateUniqueMovieCode(ctx context.Context) (string, error) {
	for i := 0; i < 100; i++ { // Maksimal 100 marta urinish
		num, err := rand.Int(rand.Reader, big.NewInt(9000))
		if err != nil {
			return "", err
		}
		code := fmt.Sprintf("MOV%d", num.Int64()+1000) // 1000 dan 9999 gacha format

		var exists bool
		err = r.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM movies WHERE code=$1)", code).Scan(&exists)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", errors.New("failed to generate unique code: collision limit reached")
}

func (r *PostgresRepository) SaveMovie(ctx context.Context, instagramURL, tgFileID, caption, code string) error {
	query := `INSERT INTO movies (instagram_url, tg_file_id, caption, code)
			  VALUES ($1, $2, $3, $4)
			  ON CONFLICT (instagram_url)
			  DO UPDATE SET tg_file_id = $2, caption = $3`
	_, err := r.Pool.Exec(ctx, query, instagramURL, tgFileID, caption, code)
	return err
}

func (r *PostgresRepository) GetMovieByCode(ctx context.Context, code string) (int, string, string, error) {
	query := `SELECT id, tg_file_id, caption FROM movies WHERE code = $1`
	var id int
	var fileID, caption string
	err := r.Pool.QueryRow(ctx, query, code).Scan(&id, &fileID, &caption)
	return id, fileID, caption, err
}

func (r *PostgresRepository) SaveReel(ctx context.Context, reelLink string, movieID int) error {
	query := `INSERT INTO reels (reel_link, movie_id) VALUES ($1, $2)
			  ON CONFLICT (reel_link) DO UPDATE SET movie_id = $2`
	_, err := r.Pool.Exec(ctx, query, reelLink, movieID)
	return err
}

func (r *PostgresRepository) GetMovieByReelLink(ctx context.Context, reelLink string) (string, string, error) {
	query := `SELECT m.tg_file_id, m.caption FROM movies m
			  JOIN reels r ON r.movie_id = m.id
			  WHERE r.reel_link = $1`
	var fileID, caption string
	err := r.Pool.QueryRow(ctx, query, reelLink).Scan(&fileID, &caption)
	return fileID, caption, err
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
              ON CONFLICT (id) DO UPDATE SET username = $2, language_code = $3`
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

func (r *PostgresRepository) GetLatestMoviesList(ctx context.Context, limit int) ([]string, error) {
	query := `SELECT code, caption FROM movies ORDER BY created_at DESC LIMIT $1`
	rows, err := r.Pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []string
	idx := 1
	for rows.Next() {
		var code, caption string
		if err := rows.Scan(&code, &caption); err != nil {
			return nil, err
		}
		if len(caption) > 30 {
			caption = caption[:27] + "..."
		}
		list = append(list, fmt.Sprintf("%d. %s — %s", idx, code, caption))
		idx++
	}
	return list, nil
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
