# Lead Scout Product Spec

## Summary

Lead Scout is a personal lead pipeline for finding high-fit freelance gigs and early-stage founders who need help turning AI/no-code prototypes into production software. It collects public/API-accessible leads, dedupes them, scores them, and sends Telegram alerts or digests for manual review.

The project optimizes for five good conversations per week, not maximum lead volume.

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

Phase 1 collectors:

- Hacker News Algolia API for freelance/project intent.
- Braintrust public jobs JSON.
- RemoteOK public API.
- WeWorkRemotely RSS.
- Reddit OAuth for founder-intent subreddits and searches.

Phase 2 founder-intent expansion:

- Vibe-coding and no-code subreddits with production-readiness pain.
- r/startups "Looking For A Cofounder" and similar founder-intent filters.
- Keyword alerts for "built this in Lovable", "take it to production", "burned through credits", "fix one thing and another breaks", "should I hire a developer", and "Supabase RLS".

Manual inbound checklist:

- Lovable Experts.
- VibeCodeFixers.
- Fiverr "vibe code fixer" style listings.
- Relevant founder/no-code communities where manual participation is allowed.

Phase 3 optional enrichment:

- LinkedIn read-assist from a human-controlled Windows browser session.
- X paid API for narrow build-in-public intent searches.
- Manual Discord/community notes only.

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
- Never use Discord self-bots.
- Never use logged-in X scraping.
- Use Reddit OAuth with a clear user agent.
- Use paid X API access if X is included.
- Prefer official APIs, RSS, or public JSON.
- Respect source rate limits and failures.
- Store enough source context to audit why a lead appeared.

## Build Phases

### Phase 1: Weekend V1

Build the zero-browser-automation pipeline:

- API/RSS/OAuth collectors for HN, Braintrust, RemoteOK, WeWorkRemotely, and Reddit.
- Postgres storage for raw items, normalized leads, scores, events, and digests.
- Heuristic gate followed by NVIDIA scoring for qualified leads.
- Telegram hot alerts for gigs and daily founder digest.
- Telegram state buttons for save, reject, approach, reply, call, won, and lost.

### Phase 2: Founder-Intent Classifier

Make the founder lane more useful:

- Add explicit intent labels for no-code/AI-builder rescue signals.
- Expand Reddit search coverage around Lovable, Bolt, Supabase, auth, deployment, and production-readiness pain.
- Track manual inbound channels such as Lovable Experts, VibeCodeFixers, and Fiverr listings.
- Use outcome events to refine scoring prompts and heuristic weights.

### Phase 3: Risk-Aware Enrichment

Add optional enrichment without making risky browsing the backbone:

- Support manually triggered LinkedIn read-assist on Windows Chrome/Playwright.
- Add X paid-API collector for narrow build-in-public queries if the ROI justifies cost.
- Keep Discord as manual presence only.
- Record enrichment metadata separately from original source data.

## Acceptance Criteria

Phase 1:

- `go test ./...` passes.
- `lead-scout collect --source hn` stores raw items and normalized leads.
- `lead-scout collect --all` can skip unconfigured Reddit credentials without breaking other sources.
- `lead-scout score --since 24h` stores heuristic and NVIDIA scoring results.
- A high-score gig lead can produce a Telegram hot alert.
- `lead-scout digest --daily` excludes rejected leads and includes qualified founder leads.
- Telegram actions record lead state transitions.
- No code path sends outreach to prospects.

Phase 2:

- Founder leads can be labeled with specific rescue signals.
- Reddit searches can target configurable intent keywords and subreddits.
- Manual inbound marketplace leads can be recorded and scored.
- Saved/rejected/won/lost outcomes are available for scoring review.

Phase 3:

- LinkedIn enrichment requires a manual, user-controlled session.
- X collection uses API credentials and logs cost-sensitive query scope.
- Discord has no automated login, reading, posting, or self-bot code path.
