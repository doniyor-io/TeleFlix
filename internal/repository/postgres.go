package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"time"

	"tg-movie-bot/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	Pool *pgxpool.Pool
}

func NewPostgresRepository(databaseURL string) (*PostgresRepository, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	repo := &PostgresRepository{
		Pool: pool,
	}

	if err := repo.migrate(ctx); err != nil {
		return nil, err
	}

	return repo, nil
}

func (r *PostgresRepository) migrate(ctx context.Context) error {
	queries := []string{

		`
		CREATE TABLE IF NOT EXISTS users (
			id BIGINT PRIMARY KEY,
			username TEXT,
			language_code VARCHAR(10) DEFAULT 'uz',
			created_at TIMESTAMP DEFAULT NOW()
		);
		`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS language_code VARCHAR(10) DEFAULT 'uz';`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS phone_number TEXT;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS first_name TEXT;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS last_name TEXT;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT NOW();`,

		`
		CREATE TABLE IF NOT EXISTS channels (
			id BIGSERIAL PRIMARY KEY,
			telegram_channel_id BIGINT UNIQUE NOT NULL,
			invite_link TEXT NOT NULL,
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT NOW()
		);
		`,
		`ALTER TABLE channels ADD COLUMN IF NOT EXISTS telegram_channel_id BIGINT;`,
		`ALTER TABLE channels ADD COLUMN IF NOT EXISTS invite_link TEXT;`,
		`ALTER TABLE channels ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT TRUE;`,
		`ALTER TABLE channels ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT NOW();`,
		`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'channels' AND column_name = 'tg_channel_id'
			) THEN
				UPDATE channels SET telegram_channel_id = tg_channel_id
				WHERE telegram_channel_id IS NULL AND tg_channel_id IS NOT NULL;
			END IF;
		END $$;
		`,
		`CREATE UNIQUE INDEX IF NOT EXISTS channels_telegram_channel_id_idx ON channels (telegram_channel_id);`,

		`
		CREATE TABLE IF NOT EXISTS movies (
			id BIGSERIAL PRIMARY KEY,
			code VARCHAR(30) UNIQUE NOT NULL,
			telegram_file_id TEXT NOT NULL,
			caption TEXT,
			created_at TIMESTAMP DEFAULT NOW()
		);
		`,
		`ALTER TABLE movies ADD COLUMN IF NOT EXISTS code VARCHAR(30);`,
		`ALTER TABLE movies ADD COLUMN IF NOT EXISTS telegram_file_id TEXT;`,
		`ALTER TABLE movies ADD COLUMN IF NOT EXISTS caption TEXT;`,
		`ALTER TABLE movies ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT NOW();`,
		`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'movies' AND column_name = 'tg_file_id'
			) THEN
				UPDATE movies SET telegram_file_id = tg_file_id
				WHERE telegram_file_id IS NULL AND tg_file_id IS NOT NULL;
			END IF;
		END $$;
		`,
		`CREATE UNIQUE INDEX IF NOT EXISTS movies_code_idx ON movies (code);`,

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
		`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'reels' AND column_name = 'reel_link'
			) THEN
				UPDATE reels SET reel_url = reel_link
				WHERE reel_url IS NULL AND reel_link IS NOT NULL;
			END IF;
		END $$;
		`,
		`UPDATE reels SET shortcode = regexp_replace(reel_url, '^.*/(reel|reels|p)/([^/?#]+).*$','\2') WHERE shortcode IS NULL AND reel_url IS NOT NULL;`,
		`CREATE UNIQUE INDEX IF NOT EXISTS reels_shortcode_idx ON reels (shortcode);`,
	}

	for _, query := range queries {
		if _, err := r.Pool.Exec(ctx, query); err != nil {
			return err
		}
	}

	return nil
}

func (r *PostgresRepository) SaveUserLanguage(
	ctx context.Context,
	userID int64,
	username string,
	lang string,
) error {

	query := `
	INSERT INTO users (
		id,
		username,
		language_code
	)
	VALUES ($1, $2, $3)
	ON CONFLICT(id)
	DO UPDATE SET
		username = EXCLUDED.username,
		language_code = EXCLUDED.language_code
	`

	_, err := r.Pool.Exec(
		ctx,
		query,
		userID,
		username,
		lang,
	)

	return err
}

func (r *PostgresRepository) UserExists(ctx context.Context, userID int64) (bool, error) {
	var exists bool
	err := r.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists)
	return exists, err
}

func (r *PostgresRepository) SaveUserContact(
	ctx context.Context,
	userID int64,
	username string,
	phoneNumber string,
	firstName string,
	lastName string,
) error {

	query := `
	INSERT INTO users (
		id,
		username,
		phone_number,
		first_name,
		last_name
	)
	VALUES ($1, $2, $3, $4, $5)
	ON CONFLICT(id)
	DO UPDATE SET
		username = EXCLUDED.username,
		phone_number = EXCLUDED.phone_number,
		first_name = EXCLUDED.first_name,
		last_name = EXCLUDED.last_name
	`

	_, err := r.Pool.Exec(ctx, query, userID, username, phoneNumber, firstName, lastName)
	return err
}

func (r *PostgresRepository) GetUserLanguage(
	ctx context.Context,
	userID int64,
) (string, error) {

	query := `
	SELECT language_code
	FROM users
	WHERE id = $1
	`

	var lang string

	err := r.Pool.QueryRow(
		ctx,
		query,
		userID,
	).Scan(&lang)

	if err != nil {
		return "uz", nil
	}

	return lang, nil
}

func (r *PostgresRepository) GenerateMovieCode() (string, error) {
	const letters = "0123456789"

	var builder strings.Builder

	builder.WriteString("MOV")

	for i := 0; i < 4; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", err
		}

		builder.WriteByte(letters[n.Int64()])
	}

	return builder.String(), nil
}

func (r *PostgresRepository) CreateMovie(
	ctx context.Context,
	fileID string,
	caption string,
) (*model.Movie, error) {
	query := `
	INSERT INTO movies (
		code,
		telegram_file_id,
		caption
	)
	VALUES ($1, $2, $3)
	RETURNING id, created_at
	`

	var movie model.Movie

	movie.TelegramFileID = fileID
	movie.Caption = caption

	for attempt := 0; attempt < 10; attempt++ {
		code, err := r.GenerateMovieCode()
		if err != nil {
			return nil, err
		}

		movie.MovieCode = code
		err = r.Pool.QueryRow(
			ctx,
			query,
			code,
			fileID,
			caption,
		).Scan(
			&movie.ID,
			&movie.CreatedAt,
		)

		if err == nil {
			return &movie, nil
		}

		if !strings.Contains(err.Error(), "duplicate key") {
			return nil, err
		}
	}

	return nil, errors.New("failed to generate a unique movie code")
}

func (r *PostgresRepository) GetMovieByCode(
	ctx context.Context,
	code string,
) (*model.Movie, error) {

	query := `
	SELECT
		id,
		code,
		telegram_file_id,
		caption,
		created_at
	FROM movies
	WHERE code = $1
	`

	var movie model.Movie

	err := r.Pool.QueryRow(
		ctx,
		query,
		code,
	).Scan(
		&movie.ID,
		&movie.MovieCode,
		&movie.TelegramFileID,
		&movie.Caption,
		&movie.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &movie, nil
}

func (r *PostgresRepository) CreateReel(
	ctx context.Context,
	shortcode string,
	reelURL string,
	movieID int64,
) error {

	query := `
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
	`

	_, err := r.Pool.Exec(
		ctx,
		query,
		shortcode,
		reelURL,
		movieID,
	)

	return err
}

func (r *PostgresRepository) GetMovieByShortcode(
	ctx context.Context,
	shortcode string,
) (*model.Movie, error) {

	query := `
	SELECT
		m.id,
		m.code,
		m.telegram_file_id,
		m.caption,
		m.created_at
	FROM reels r
	INNER JOIN movies m
	ON r.movie_id = m.id
	WHERE r.shortcode = $1
	`

	var movie model.Movie

	err := r.Pool.QueryRow(
		ctx,
		query,
		shortcode,
	).Scan(
		&movie.ID,
		&movie.MovieCode,
		&movie.TelegramFileID,
		&movie.Caption,
		&movie.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &movie, nil
}

func (r *PostgresRepository) AddChannel(
	ctx context.Context,
	channelID int64,
	inviteLink string,
) error {
	channelID = normalizeStoredChannelID(channelID)

	query := `
	INSERT INTO channels (
		telegram_channel_id,
		invite_link
	)
	VALUES ($1, $2)
	ON CONFLICT(telegram_channel_id)
	DO UPDATE SET
		invite_link = EXCLUDED.invite_link,
		is_active = true
	`

	_, err := r.Pool.Exec(
		ctx,
		query,
		channelID,
		inviteLink,
	)

	return err
}

func (r *PostgresRepository) GetChannels(
	ctx context.Context,
) ([]model.Channel, error) {

	query := `
	SELECT
		id,
		telegram_channel_id,
		invite_link,
		is_active,
		created_at
	FROM channels
	WHERE is_active = true
	`

	rows, err := r.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var channels []model.Channel

	for rows.Next() {
		var channel model.Channel

		err := rows.Scan(
			&channel.ID,
			&channel.TelegramChannelID,
			&channel.InviteLink,
			&channel.IsActive,
			&channel.CreatedAt,
		)

		if err != nil {
			return nil, err
		}

		channels = append(channels, channel)
	}

	return channels, nil
}

func (r *PostgresRepository) DeleteChannel(
	ctx context.Context,
	channelID int64,
) error {

	query := `
	DELETE FROM channels
	WHERE telegram_channel_id = $1
	`

	result, err := r.Pool.Exec(
		ctx,
		query,
		channelID,
	)

	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("channel not found")
	}

	return nil
}

func (r *PostgresRepository) GetMovies(
	ctx context.Context,
) ([]model.Movie, error) {

	query := `
	SELECT
		id,
		code,
		telegram_file_id,
		caption,
		created_at,
		(
			SELECT reel_url
			FROM reels
			WHERE reels.movie_id = movies.id
			ORDER BY created_at DESC
			LIMIT 1
		) AS reel_url
	FROM movies
	ORDER BY created_at DESC
	`

	rows, err := r.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var movies []model.Movie

	for rows.Next() {
		var movie model.Movie
		var reelURL sql.NullString

		err := rows.Scan(
			&movie.ID,
			&movie.MovieCode,
			&movie.TelegramFileID,
			&movie.Caption,
			&movie.CreatedAt,
			&reelURL,
		)

		if err != nil {
			return nil, err
		}

		if reelURL.Valid {
			movie.ReelURL = reelURL.String
		}

		movies = append(movies, movie)
	}

	return movies, nil
}

func (r *PostgresRepository) GetStatistics(
	ctx context.Context,
) (map[string]int64, error) {

	stats := make(map[string]int64)

	var users int64
	var movies int64
	var reels int64

	err := r.Pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM users`,
	).Scan(&users)

	if err != nil {
		return nil, err
	}

	err = r.Pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM movies`,
	).Scan(&movies)

	if err != nil {
		return nil, err
	}

	err = r.Pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM reels`,
	).Scan(&reels)

	if err != nil {
		return nil, err
	}

	stats["users"] = users
	stats["movies"] = movies
	stats["reels"] = reels

	return stats, nil
}

func ExtractShortcode(instagramURL string) string {
	instagramURL = strings.TrimSpace(instagramURL)
	instagramURL = strings.TrimSuffix(instagramURL, "/")

	parts := strings.Split(instagramURL, "/")

	for i, part := range parts {
		if (part == "reel" || part == "reels" || part == "p") && i+1 < len(parts) {
			return strings.Split(parts[i+1], "?")[0]
		}
	}

	return ""
}

func normalizeStoredChannelID(channelID int64) int64 {
	if channelID <= 0 {
		return channelID
	}

	normalized, err := strconv.ParseInt("-100"+strconv.FormatInt(channelID, 10), 10, 64)
	if err != nil {
		return channelID
	}

	return normalized
}
