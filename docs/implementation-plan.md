# Lead Scout Implementation Plan

## Architecture

Lead Scout is a small Go CLI/service with these packages:

- `cmd/lead-scout`: CLI entrypoint.
- `internal/config`: environment config.
- `internal/core`: shared domain types and interfaces.
- `internal/db`: migrations and repository methods.
- `internal/collectors`: API/RSS/OAuth collectors.
- `internal/normalize`: shared normalizer and dedupe helpers.
- `internal/scoring`: heuristic gate and NVIDIA API scorer.
- `internal/telegram`: Telegram notifier and bot actions.
- `internal/app`: command orchestration.

The first version runs as commands. Pandora can schedule those commands through systemd timers while `lead-scout bot` runs as a service.

## Database

Use Docker Postgres with these tables:

- `sources`: source metadata.
- `raw_items`: source payloads, fetched unchanged.
- `leads`: normalized deduped leads.
- `lead_scores`: score, rationale, opener, prompt version, model.
- `lead_events`: state transitions.
- `digests`: generated digest payloads.

Deduping uses canonical URL, then `source + external_id`, then content hash.

## Commands

- `lead-scout migrate`: apply idempotent schema migrations.
- `lead-scout collect --source hn`: fetch one source.
- `lead-scout collect --all`: fetch all configured sources.
- `lead-scout score --since 24h`: score recently created leads.
- `lead-scout digest --daily`: send daily digest candidates.
- `lead-scout bot`: poll Telegram callbacks and record state transitions.
- `lead-scout serve --addr :8080`: serve JSON API, OpenAPI spec, and Scalar docs.

## Scoring

The heuristic scorer runs first and decides whether a lead is worth an LLM call. NVIDIA scoring receives the normalized lead plus scoring rubric and returns JSON:

```json
{
  "score": 82,
  "category": "gig",
  "rationale": "Strong AI SaaS contract fit with budget signal.",
  "draft_opener": "Short manual opener for the user to edit.",
  "should_notify": true
}
```

If NVIDIA is unavailable or returns invalid JSON, the heuristic result is stored instead.

## Telegram

Telegram is the only v1 UI.

- Hot gig alerts include title, score, source, URL, rationale, and action buttons.
- Daily founder digest includes qualified non-rejected founder leads.
- Buttons update local lead state only.
- No prospect-facing action exists.

## Deployment

Local:

```powershell
docker compose up -d
go run ./cmd/lead-scout migrate
go run ./cmd/lead-scout collect --all
go run ./cmd/lead-scout score --since 24h
go run ./cmd/lead-scout digest --daily
```

Pandora:

- Install Docker and Go or copy a built Linux binary.
- Store `.env` in the service working directory.
- Run Postgres via Docker Compose.
- Run `lead-scout bot` as a systemd service.
- Run collect/score/digest commands via systemd timers.
- Back up with `pg_dump`.

## Tests

- Unit tests for config, normalizers, dedupe helpers, heuristic scoring, Telegram callback parsing, and state transitions.
- Repository integration tests can be added against Docker Postgres after v1 stabilizes.
- Live collector smoke tests stay manual to avoid flaky normal test runs.
