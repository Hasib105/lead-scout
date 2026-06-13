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

## Build Phases

### Phase 1: Weekend V1

Scope:

- API-first collectors for HN, Braintrust, RemoteOK, WeWorkRemotely, and Reddit.
- Postgres-backed raw item, lead, score, event, and digest storage.
- Heuristic gate plus NVIDIA deep scoring.
- Telegram hot alerts for high-score gigs.
- Daily Telegram digest for qualified founder leads.
- Telegram callback handling for local CRM-lite state changes.

Exit criteria:

- `go test ./...` passes.
- `collect --all`, `score --since 24h`, and `digest --daily` work with configured credentials.
- Missing optional credentials, such as Reddit, do not break other collectors.
- No code path sends automated outreach.

### Phase 2: Founder-Intent Classifier

Scope:

- Add explicit founder rescue signal extraction during normalization or scoring.
- Track signals such as Lovable/Bolt prototypes, production-readiness requests, Supabase RLS, auth/deployment pain, credit burn, and "hire a developer" intent.
- Add configurable Reddit searches and subreddit lists for founder/no-code/vibe-coding communities.
- Add manual inbound source support for Lovable Experts, VibeCodeFixers, Fiverr, and similar listings.
- Review outcome feedback from lead state transitions and tune scoring rubrics.

Exit criteria:

- Founder leads can show which intent signals made them qualify.
- Digest entries include signal-specific rationale.
- Manual inbound leads can be inserted, scored, and tracked through the same state machine.
- Scoring review can compare accepted/rejected outcomes against original score reasons.

### Phase 3: Risk-Aware Enrichment

Scope:

- Add optional LinkedIn read-assist that runs only through a human-controlled Windows browser workflow.
- Add optional X collector through paid API credentials and narrow intent queries.
- Keep Discord as manual presence only, with no self-bot or logged-in scraping.
- Store enrichment data separately from source payloads so original lead provenance remains auditable.
- Add configuration flags so risky or paid enrichment is disabled by default.

Exit criteria:

- LinkedIn enrichment cannot post, message, connect, or browse without user action.
- X collection uses API credentials, records query scope, and can be disabled independently.
- No Discord automation exists.
- Phase 1 collectors continue to work without any enrichment setup.

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
