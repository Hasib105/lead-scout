# Lead Scout Product Spec

## Summary

Lead Scout is a personal lead pipeline for finding high-fit freelance gigs and early-stage founders who need help turning AI/no-code prototypes into production software. It collects public/API-accessible leads, dedupes them, scores them, and sends Telegram alerts or digests for manual review.

V1 optimizes for five good conversations per week, not maximum lead volume.

## Goals

- Find contract/freelance/project work with strong budget and urgency.
- Find founders with "vibe-code rescue" signals: stuck Lovable/Bolt/no-code prototypes, production readiness, Supabase/RLS/auth/deployment pain, or need for a real engineer.
- Alert quickly for gig leads that decay in hours.
- Batch founder leads into a daily digest.
- Keep all outreach manual and human-approved.

## Non-Goals

- No automated DMs, connection requests, emails, or comments.
- No LinkedIn automation in v1.
- No Discord self-bots or logged-in scraping.
- No web UI in v1.
- No broad job-search product in v1.

## Target Leads

Default quality filters:

- Remote US/EU-friendly work.
- $75+/hr or equivalent premium project budget.
- Time-bound deliverables.
- SaaS, AI, agentic systems, production rescue, integrations, backend, data, or infrastructure-heavy work.
- Founders before VC-scale hiring, especially non-technical founders or prototypes stuck after no-code/AI-builder tools.

## Sources

V1 collectors:

- Hacker News Algolia API for freelance/project intent.
- Braintrust public jobs JSON.
- RemoteOK public API.
- WeWorkRemotely RSS.
- Reddit OAuth for founder-intent subreddits and searches.

Manual inbound checklist:

- Lovable Experts.
- VibeCodeFixers.
- Fiverr "vibe code fixer" style listings.
- Relevant founder/no-code communities where manual participation is allowed.

## Workflow

1. Collectors fetch raw public/API data and store unmodified payloads.
2. Normalizers convert raw items into a shared lead format.
3. Dedupe prevents repeated alerts across sources.
4. Heuristic gate filters obvious noise.
5. NVIDIA API scoring evaluates high-potential leads and produces score, rationale, and a draft opener.
6. Telegram sends hot gig alerts immediately and founder digests daily.
7. User manually reviews, saves, rejects, approaches, and tracks outcomes.

## Scoring

Scores range from 0 to 100.

- 80+: hot lead, send immediate alert if gig category.
- 65-79: include in digest.
- Below 65: keep stored but do not notify by default.

Scoring dimensions:

- Fit to target work.
- Budget/rate signal.
- Urgency.
- Remote/timezone fit.
- Founder pain language.
- Clarity of next action.
- Risk/noise indicators.

## Safety Rules

- Never automate outreach.
- Never scrape logged-in LinkedIn or Discord.
- Use Reddit OAuth with a clear user agent.
- Prefer official APIs, RSS, or public JSON.
- Respect source rate limits and failures.
- Store enough source context to audit why a lead appeared.

## Acceptance Criteria

- `go test ./...` passes.
- `lead-scout collect --source hn` stores raw items and normalized leads.
- `lead-scout collect --all` can skip unconfigured Reddit credentials without breaking other sources.
- `lead-scout score --since 24h` stores heuristic and NVIDIA scoring results.
- A high-score gig lead can produce a Telegram hot alert.
- `lead-scout digest --daily` excludes rejected leads and includes qualified founder leads.
- Telegram actions record lead state transitions.
- No code path sends outreach to prospects.
