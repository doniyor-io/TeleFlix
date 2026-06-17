package instagramwebhook

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) EnsureSchema(ctx context.Context) error {
	queries := []string{
		`
		CREATE TABLE IF NOT EXISTS reels (
			id BIGSERIAL PRIMARY KEY,
			shortcode VARCHAR(100) UNIQUE NOT NULL,
			reel_url TEXT NOT NULL,
			movie_id BIGINT REFERENCES movies(id) ON DELETE CASCADE,
			created_at TIMESTAMP DEFAULT NOW()
		);
		`,
		`ALTER TABLE reels ADD COLUMN IF NOT EXISTS shortcode VARCHAR(100);`,
		`ALTER TABLE reels ADD COLUMN IF NOT EXISTS reel_url TEXT;`,
		`ALTER TABLE reels ADD COLUMN IF NOT EXISTS movie_id BIGINT;`,
		`ALTER TABLE reels ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT NOW();`,
		`CREATE UNIQUE INDEX IF NOT EXISTS reels_shortcode_idx ON reels (shortcode);`,
	}

	for _, query := range queries {
		if _, err := s.pool.Exec(ctx, query); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) BindReelToMovieCode(ctx context.Context, code string, shortcode string, reelURL string) (int64, error) {
	code = strings.TrimSpace(code)
	shortcode = strings.TrimSpace(shortcode)
	reelURL = strings.TrimSpace(reelURL)

	if code == "" {
		return 0, errors.New("movie code is required")
	}

	if shortcode == "" {
		return 0, errors.New("instagram shortcode is required")
	}

	if reelURL == "" {
		return 0, errors.New("reel url is required")
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var movieID int64
	err = tx.QueryRow(ctx, `SELECT id FROM movies WHERE code = $1`, code).Scan(&movieID)
	if err != nil {
		return 0, err
	}

	_, err = tx.Exec(
		ctx,
		`
		INSERT INTO reels (
			shortcode,
			reel_url,
			movie_id
		)
		VALUES ($1, $2, $3)
		ON CONFLICT(shortcode)
		DO UPDATE SET
			reel_url = EXCLUDED.reel_url,
			movie_id = EXCLUDED.movie_id
		`,
		shortcode,
		reelURL,
		movieID,
	)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	return movieID, nil
}
