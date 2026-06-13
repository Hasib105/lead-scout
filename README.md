# lead-scout

Personal lead-generation pipeline for premium freelance gigs and early-stage founder rescue work.

The roadmap has three phases. Phase 1 is the working API-first baseline, Phase 2 adds better founder-intent detection, and Phase 3 adds optional risk-aware enrichment.

## Roadmap

### Phase 1: Weekend V1

Intentionally small, zero-browser-automation baseline:

- API-first collectors only.
- Hacker News, Braintrust, RemoteOK, WeWorkRemotely, and Reddit collectors.
- Docker Postgres for storage.
- NVIDIA API scoring after a heuristic gate.
- Fast gig alerts and daily founder digests through Telegram.
- No automated outreach, LinkedIn automation, Discord self-bots, or logged-in scraping.

### Phase 2: Founder-Intent Classifier

Improve the "vibe-code rescue" lane:

- Monitor founder/no-code/vibe-coding communities for production-readiness pain.
- Classify signals such as Lovable/Bolt prototypes, Supabase RLS, burned credits, auth/deployment issues, and "should I hire a developer" language.
- Add inbound marketplace tracking for Lovable Experts, VibeCodeFixers, Fiverr, and similar manual-listing channels.
- Feed outcome feedback from saved/rejected/won/lost leads back into scoring rubrics.

### Phase 3: Risk-Aware Enrichment

Optional enrichment only where it is worth the platform risk:

- LinkedIn read-assist through a human-controlled Windows browser session, never automated writes.
- X build-in-public queries through paid API access only, not logged-in scraping.
- Manual Discord/community presence tracking without self-bots.
- Keep enrichment separate from collection so the core pipeline remains API-first and resilient.

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
