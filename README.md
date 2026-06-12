# lead-scout

Personal lead-generation pipeline for premium freelance gigs and early-stage founder rescue work.

V1 is intentionally small:

- API-first collectors only.
- Docker Postgres for storage.
- NVIDIA API scoring after a heuristic gate.
- Telegram alerts and digests.
- No automated outreach, LinkedIn automation, Discord self-bots, or logged-in scraping.

## Requirements

- Go 1.25+
- Docker
- Postgres via `docker compose`
- Telegram bot token and chat ID
- Reddit OAuth app credentials for Reddit collectors
- NVIDIA API key

## Setup

```powershell
copy .env.example .env
docker compose up -d
go mod download
go run ./cmd/lead-scout migrate
```

Set these values in `.env` or your shell:

```text
DATABASE_URL=postgres://lead_scout:lead_scout@localhost:5432/lead_scout?sslmode=disable
TELEGRAM_BOT_TOKEN=
TELEGRAM_CHAT_ID=
REDDIT_CLIENT_ID=
REDDIT_CLIENT_SECRET=
REDDIT_USER_AGENT=lead-scout/0.1 by u_your_username
NVIDIA_API_KEY=
NVIDIA_MODEL=google/gemma-4-31b-it
NVIDIA_BASE_URL=https://integrate.api.nvidia.com/v1/chat/completions
API_ADDR=:8080
```

## Commands

```powershell
go run ./cmd/lead-scout migrate
go run ./cmd/lead-scout collect --source hn
go run ./cmd/lead-scout collect --all
go run ./cmd/lead-scout score --since 24h
go run ./cmd/lead-scout digest --daily
go run ./cmd/lead-scout bot
go run ./cmd/lead-scout serve --addr :8080
```

Useful source names:

- `hn`
- `braintrust`
- `remoteok`
- `wwr`
- `reddit`

## Local Development

Run tests:

```powershell
go test ./...
```

Run smoke collectors manually when you want live network checks:

```powershell
go run ./cmd/lead-scout collect --source remoteok
```

OpenAPI and Scalar docs:

```powershell
go run ./cmd/lead-scout serve --addr :8080
```

Then open:

- `http://localhost:8080/docs` for Scalar API docs and browser testing.
- `http://localhost:8080/openapi.json` for the raw OpenAPI spec.

## Pandora Deployment

The deployment target is a small Ubuntu server. Run Postgres through Docker Compose, build the Go binary, and run either:

- a long-running systemd service for `lead-scout bot`, plus timers for collect/score/digest commands, or
- one long-running scheduler process added later once v1 behavior is stable.

Keep `.env` outside Git. Back up Postgres with `pg_dump` before changing schema or moving hosts.
