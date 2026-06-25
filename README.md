# cinemagobot

cinemagobot is a Go backend for a Telegram movie discovery bot. The idea is simple from the user's point of view: they paste an Instagram Reel link into Telegram, and the bot sends back the right movie.

Behind that small interaction there is a production flow for channel owners, movie admins, Telegram users, and Instagram Reels. This repository contains the backend that ties those pieces together.

## What This Project Does

The bot stores movies in PostgreSQL with a unique movie code. A Reel can then be linked to that movie, either from the admin tools or automatically through Meta Instagram Graph API webhooks.

A typical flow looks like this:

1. An admin adds a movie to the system.
2. The movie receives a unique code, for example `9988` or `movie777`.
3. The channel owner publishes an Instagram Reel and includes that code as a hashtag in the caption, for example `#9988`.
4. Meta sends a webhook event to the backend when the Reel is published.
5. The backend reads the Reel permalink and caption, extracts the code, finds the matching movie, and stores the Reel-to-movie mapping.
6. A Telegram user sends the Reel link to the bot.
7. The bot extracts the Instagram shortcode from the link and immediately returns the matching movie.

No scraping is needed for the automated Reel ingestion path. The system is designed around push events from Meta, which is faster, cleaner, and much easier to operate.

## Main Components

The repository is intentionally small and direct:

```text
cmd/bot/                 Main Telegram bot and admin API server
cmd/instagram-webhook/   Isolated Instagram webhook server
config/                  Application configuration loader for the bot
internal/bot/            Telegram handlers, workers, localization logic
internal/instagramwebhook/ Typed Meta webhook ingestion and DB binding
internal/model/          Shared data models
internal/repository/     PostgreSQL and Redis repositories
pkg/telegram/            Telegram API client
locales/                 Bot translations
```

The Telegram bot code and the Instagram webhook code are separated. That matters because the Telegram package is already production-critical, and the Instagram webhook listener can evolve without disturbing the bot handlers.

## Tech Stack

- Go 1.25
- PostgreSQL
- Redis
- Telegram Bot API
- Meta Instagram Graph API Webhooks
- Docker Compose for local/prod-style deployment

## Environment Variables

The app reads configuration from `.env`. Keep real tokens out of commits and logs.

Required for the Telegram bot:

```env
PORT=9090
ENV=development
PUBLIC_URL=https://your-public-url.example
TELEGRAM_BOT_TOKEN=your-telegram-bot-token
ADMIN_IDS=123456789,987654321
DATABASE_URL=postgres://postgres:linux@postgres:5432/moviedb?sslmode=disable
REDIS_URL=redis:6379
# Optional: set these only if webhook and frontend URLs differ from PUBLIC_URL.
WEBHOOK_URL=https://your-public-url.example
FRONTEND_URL=https://your-public-url.example
BOT_NAME=Movie Finder Bot
BRIDGE_SECRET=change-this-secret
```

Required for Instagram webhook automation:

```env
META_WEBHOOK_VERIFY_TOKEN=change-this-verify-token
INSTAGRAM_ACCESS_TOKEN=your-instagram-graph-token
INSTAGRAM_BUSINESS_ID=your-instagram-business-id
META_WEBHOOK_SECRET=change-this-meta-secret
```

Useful for local tunneling and frontend proxying:

```env
FRONTEND_PORT=3000
FRONTEND_INTERNAL_URL=http://frontend:80
NGROK_URL=https://your-ngrok-url.example
```

## Running With Docker Compose

The fastest way to run the normal bot stack is:

```bash
docker compose up --build
```

This starts:

- PostgreSQL on `5432`
- Redis on `6379`
- the Go bot backend on `9090`
- the frontend container from `../movie-linker-tma`

The backend exposes:

```text
POST /webhook
GET  /health
GET  /api/admin/stats
GET  /api/admin/movies
POST /api/admin/movies
POST /api/admin/movies/delete
POST /api/admin/movies/link-reel
GET  /api/admin/movies/top
GET  /api/admin/channels
POST /api/admin/channels
POST /api/admin/channels/delete
GET  /api/admin/users
```

Any other path is proxied to the frontend.

## Local Development

Install dependencies:

```bash
go mod tidy
```

Run the bot server directly:

```bash
go run ./cmd/bot
```

Run tests:

```bash
go test ./...
```

Build everything:

```bash
go build ./...
```

## Telegram Webhook Setup

Telegram needs a public HTTPS URL. For local development, this project includes an automation script that starts ngrok, updates `.env`, rebuilds Docker containers, checks health, and registers the Telegram webhook.

```bash
./automation.sh
```

The script registers:

```text
https://your-public-url/webhook
```

as the Telegram webhook endpoint.

## Instagram Webhook Server

The Instagram webhook implementation lives in a separate command:

```bash
go run ./cmd/instagram-webhook
```

It listens on:

```text
GET  /webhook/instagram
POST /webhook/instagram
```

By default it uses `PORT=9090`, the same as the main bot server. Run only one of them on that port at a time, or override `PORT` when running locally:

```bash
PORT=9091 go run ./cmd/instagram-webhook
```

For Meta, the public callback URL should point to:

```text
https://your-public-url/webhook/instagram
```

### Verification Request

Meta verifies the webhook with a `GET` request containing:

```text
hub.mode=subscribe
hub.verify_token=your-token
hub.challenge=some-challenge-value
```

The server compares `hub.verify_token` with `META_WEBHOOK_VERIFY_TOKEN`. If it matches, it returns the challenge with `200 OK`.

### Ingestion Request

Meta sends Reel events through `POST /webhook/instagram`.

The webhook code:

- decodes the nested Meta payload into typed Go structs
- reads the Reel `permalink`
- reads the Reel `caption`
- extracts the first clean alphanumeric hashtag code
- extracts the Instagram shortcode from the permalink
- opens a PostgreSQL transaction
- finds the movie in `movies` by `code`
- upserts the mapping into `reels`

Example caption:

```text
Best scene today #movie777
```

Extracted movie code:

```text
movie777
```

Example permalink:

```text
https://www.instagram.com/reel/C9abc123/
```

Extracted shortcode:

```text
C9abc123
```

## Database Tables

The repository creates and maintains the required tables on startup.

Important tables:

```text
users       Telegram users and language/contact state
channels    Required subscription channels
movies      Stored movies and their unique lookup codes
reels       Instagram shortcode -> movie mapping
```

The `reels` table is the bridge between Instagram and Telegram:

```text
shortcode   Unique Instagram Reel/Post shortcode
reel_url    Full Instagram URL
movie_id    Reference to movies.id
```

When a Telegram user sends an Instagram URL, the bot extracts the shortcode and looks it up in `reels`.

## Admin Workflow

Admins can:

- add movies
- delete movies
- link a Reel to a movie manually
- manage required Telegram channels
- view stats
- view top movies
- inspect users

The admin API is served by the main bot process. The Telegram user flow and the admin flow share the same PostgreSQL data, so a movie linked from the admin panel and a movie linked by Instagram webhook both end up in the same lookup path.

## Production Notes

Use real secrets for:

- `TELEGRAM_BOT_TOKEN`
- `BRIDGE_SECRET`
- `META_WEBHOOK_VERIFY_TOKEN`
- `META_WEBHOOK_SECRET`
- `INSTAGRAM_ACCESS_TOKEN`

Make sure your public URL is stable. If you use ngrok locally, every new tunnel URL must be registered again with Telegram and configured in the Meta developer dashboard.

For Meta webhooks, subscribe the Instagram business account to the relevant media events in the Meta app dashboard, then set the callback URL to `/webhook/instagram`.

## Common Commands

```bash
# Format Go code
gofmt -w ./cmd ./config ./internal ./pkg

# Clean module dependencies
go mod tidy

# Run all tests
go test ./...

# Build all commands
go build ./...

# Run the main bot
go run ./cmd/bot

# Run the isolated Instagram webhook listener
go run ./cmd/instagram-webhook

# Start the Docker stack
docker compose up --build
```

## Troubleshooting

If the Telegram bot does not respond, check:

- `TELEGRAM_BOT_TOKEN` is valid
- Telegram webhook is registered to `WEBHOOK_URL + /webhook`
- the backend is reachable from the public internet
- Redis and PostgreSQL are running

If Instagram Reel mapping does not work, check:

- Meta is calling `/webhook/instagram`
- `META_WEBHOOK_VERIFY_TOKEN` matches the token configured in Meta
- the Reel caption contains a hashtag code, for example `#9988`
- the same code exists in the `movies.code` column
- the Reel permalink contains a usable `/reel/{shortcode}` or `/p/{shortcode}` path

If the app starts but cannot connect to the database, check `DATABASE_URL`. Inside Docker Compose the host is `postgres`; outside Docker it is usually `localhost`.

## Development Promise

The Telegram bot layer is treated as production-sensitive. New integrations should stay isolated unless there is a clear reason to touch the bot handlers. The Instagram webhook code follows that rule: it writes to the existing database tables, and the current Telegram lookup path keeps working as it already does.
