package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"lead-scout/internal/core"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(conn *sql.DB) *Repository {
	return &Repository{db: conn}
}

func (r *Repository) UpsertRawItem(ctx context.Context, item core.RawItem) (core.RawItem, error) {
	if item.ExternalID == "" {
		item.ExternalID = stableID(item.Source, item.URL, string(item.Payload))
	}
	if item.URL == "" {
		item.URL = "source://" + item.Source + "/" + item.ExternalID
	}

	var id int64
	var fetchedAt time.Time
	err := r.db.QueryRowContext(ctx, `
		insert into raw_items (source, external_id, url, payload, published_at)
		values ($1, $2, $3, $4, $5)
		on conflict (source, external_id) do update
			set url = excluded.url,
			    payload = excluded.payload,
			    fetched_at = now(),
			    published_at = excluded.published_at
		returning id, fetched_at
	`, item.Source, item.ExternalID, item.URL, []byte(item.Payload), item.PublishedAt).Scan(&id, &fetchedAt)
	if err != nil {
		return item, err
	}
	item.ID = id
	item.FetchedAt = fetchedAt
	return item, nil
}

func (r *Repository) UpsertLead(ctx context.Context, lead core.Lead) (core.Lead, error) {
	if lead.ExternalID == "" {
		lead.ExternalID = stableID(lead.Source, lead.URL, lead.Title, lead.Body)
	}
	if lead.CanonicalURL == "" {
		lead.CanonicalURL = lead.URL
	}
	if lead.ContentHash == "" {
		lead.ContentHash = stableID(lead.Source, lead.Title, lead.Body)
	}
	if lead.State == "" {
		lead.State = core.StateNew
	}

	err := r.db.QueryRowContext(ctx, `
		insert into leads (
			raw_item_id, source, external_id, category, title, body, url, canonical_url,
			author, company, location, compensation, posted_at, content_hash, state
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		on conflict do nothing
		returning id, created_at, updated_at
	`, nullableID(lead.RawItemID), lead.Source, lead.ExternalID, string(lead.Category), lead.Title, lead.Body, lead.URL, lead.CanonicalURL,
		lead.Author, lead.Company, lead.Location, lead.Compensation, lead.PostedAt, lead.ContentHash, string(lead.State)).
		Scan(&lead.ID, &lead.CreatedAt, &lead.UpdatedAt)
	if err == nil {
		return lead, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return lead, err
	}

	found, err := r.FindLeadDuplicate(ctx, lead)
	if err != nil {
		return lead, err
	}
	return found, nil
}

func (r *Repository) FindLeadDuplicate(ctx context.Context, lead core.Lead) (core.Lead, error) {
	query := `
		select id, raw_item_id, source, external_id, category, title, body, url, canonical_url,
		       author, company, location, compensation, posted_at, content_hash, state, created_at, updated_at
		from leads
		where canonical_url = $1 or (source = $2 and external_id = $3) or content_hash = $4
		order by created_at asc
		limit 1
	`
	return scanLead(r.db.QueryRowContext(ctx, query, lead.CanonicalURL, lead.Source, lead.ExternalID, lead.ContentHash))
}

func (r *Repository) PendingScoreLeads(ctx context.Context, since time.Duration) ([]core.Lead, error) {
	rows, err := r.db.QueryContext(ctx, `
		select l.id, l.raw_item_id, l.source, l.external_id, l.category, l.title, l.body, l.url, l.canonical_url,
		       l.author, l.company, l.location, l.compensation, l.posted_at, l.content_hash, l.state, l.created_at, l.updated_at
		from leads l
		where l.created_at >= now() - ($1 * interval '1 second')
		  and l.state <> 'rejected'
		  and not exists (select 1 from lead_scores s where s.lead_id = l.id)
		order by l.created_at desc
	`, int64(since.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var leads []core.Lead
	for rows.Next() {
		lead, err := scanLead(rows)
		if err != nil {
			return nil, err
		}
		leads = append(leads, lead)
	}
	return leads, rows.Err()
}

func (r *Repository) ListLeads(ctx context.Context, category core.Category, state core.LeadState, limit int) ([]core.LeadWithScore, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		select l.id, l.raw_item_id, l.source, l.external_id, l.category, l.title, l.body, l.url, l.canonical_url,
		       l.author, l.company, l.location, l.compensation, l.posted_at, l.content_hash, l.state, l.created_at, l.updated_at,
		       coalesce(s.id, 0), coalesce(s.lead_id, 0), coalesce(s.score, 0), coalesce(s.category, l.category),
		       coalesce(s.rationale, ''), coalesce(s.draft_opener, ''), coalesce(s.should_notify, false),
		       coalesce(s.prompt_version, ''), coalesce(s.model, ''), coalesce(s.created_at, l.created_at)
		from leads l
		left join lateral (
			select *
			from lead_scores s
			where s.lead_id = l.id
			order by s.created_at desc
			limit 1
		) s on true
		where ($1 = '' or l.category = $1)
		  and ($2 = '' or l.state = $2)
		order by l.created_at desc
		limit $3
	`, string(category), string(state), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.LeadWithScore
	for rows.Next() {
		var item core.LeadWithScore
		if err := scanLeadAndScore(rows, &item.Lead, &item.Score); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) InsertLeadScore(ctx context.Context, score core.LeadScore) (core.LeadScore, error) {
	err := r.db.QueryRowContext(ctx, `
		insert into lead_scores (lead_id, score, category, rationale, draft_opener, should_notify, prompt_version, model)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		returning id, created_at
	`, score.LeadID, score.Score, string(score.Category), score.Rationale, score.DraftOpener, score.ShouldNotify, score.PromptVersion, score.Model).
		Scan(&score.ID, &score.CreatedAt)
	return score, err
}

func (r *Repository) DigestCandidates(ctx context.Context, categories []core.Category, minScore int, limit int) ([]core.LeadWithScore, error) {
	rows, err := r.db.QueryContext(ctx, `
		select l.id, l.raw_item_id, l.source, l.external_id, l.category, l.title, l.body, l.url, l.canonical_url,
		       l.author, l.company, l.location, l.compensation, l.posted_at, l.content_hash, l.state, l.created_at, l.updated_at,
		       s.id, s.lead_id, s.score, s.category, s.rationale, s.draft_opener, s.should_notify, s.prompt_version, s.model, s.created_at
		from leads l
		join lateral (
			select *
			from lead_scores s
			where s.lead_id = l.id
			order by s.created_at desc
			limit 1
		) s on true
		where l.state in ('new', 'saved')
		  and l.category = any($1)
		  and s.score >= $2
		order by s.score desc, l.created_at desc
		limit $3
	`, categories, minScore, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.LeadWithScore
	for rows.Next() {
		var item core.LeadWithScore
		if err := scanLeadAndScore(rows, &item.Lead, &item.Score); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) HotLeadCandidates(ctx context.Context, minScore int) ([]core.LeadWithScore, error) {
	rows, err := r.db.QueryContext(ctx, `
		select l.id, l.raw_item_id, l.source, l.external_id, l.category, l.title, l.body, l.url, l.canonical_url,
		       l.author, l.company, l.location, l.compensation, l.posted_at, l.content_hash, l.state, l.created_at, l.updated_at,
		       s.id, s.lead_id, s.score, s.category, s.rationale, s.draft_opener, s.should_notify, s.prompt_version, s.model, s.created_at
		from leads l
		join lead_scores s on s.lead_id = l.id
		where l.state = 'new'
		  and l.category = 'gig'
		  and s.should_notify = true
		  and s.score >= $1
		  and not exists (
		  	select 1 from lead_events e where e.lead_id = l.id and e.event_type = 'notified'
		  )
		order by s.score desc, s.created_at desc
	`, minScore)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.LeadWithScore
	for rows.Next() {
		var item core.LeadWithScore
		if err := scanLeadAndScore(rows, &item.Lead, &item.Score); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) RecordEvent(ctx context.Context, leadID int64, eventType, note string, metadata map[string]string) error {
	if eventType == "" {
		return errors.New("event type is required")
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		insert into lead_events (lead_id, event_type, note, metadata)
		values ($1, $2, $3, $4)
	`, leadID, eventType, note, payload)
	return err
}

func (r *Repository) SetLeadState(ctx context.Context, leadID int64, state core.LeadState, note string) error {
	switch state {
	case core.StateNew, core.StateSaved, core.StateRejected, core.StateApproached, core.StateReplied, core.StateCall, core.StateWon, core.StateLost:
	default:
		return errors.New("invalid lead state")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `update leads set state = $1, updated_at = now() where id = $2`, string(state), leadID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `
		insert into lead_events (lead_id, event_type, note)
		values ($1, $2, $3)
	`, leadID, string(state), note); err != nil {
		return err
	}
	return tx.Commit()
}

func nullableID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

func stableID(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte(strings.TrimSpace(strings.ToLower(part))))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanLead(row rowScanner) (core.Lead, error) {
	var lead core.Lead
	var rawItemID sql.NullInt64
	var postedAt sql.NullTime
	var category string
	var state string
	err := row.Scan(
		&lead.ID, &rawItemID, &lead.Source, &lead.ExternalID, &category, &lead.Title, &lead.Body, &lead.URL, &lead.CanonicalURL,
		&lead.Author, &lead.Company, &lead.Location, &lead.Compensation, &postedAt, &lead.ContentHash, &state, &lead.CreatedAt, &lead.UpdatedAt,
	)
	if err != nil {
		return lead, err
	}
	if rawItemID.Valid {
		lead.RawItemID = rawItemID.Int64
	}
	if postedAt.Valid {
		lead.PostedAt = &postedAt.Time
	}
	lead.Category = core.Category(category)
	lead.State = core.LeadState(state)
	return lead, nil
}

func scanLeadAndScore(row rowScanner, lead *core.Lead, score *core.LeadScore) error {
	var rawItemID sql.NullInt64
	var postedAt sql.NullTime
	var leadCategory, state, scoreCategory string
	err := row.Scan(
		&lead.ID, &rawItemID, &lead.Source, &lead.ExternalID, &leadCategory, &lead.Title, &lead.Body, &lead.URL, &lead.CanonicalURL,
		&lead.Author, &lead.Company, &lead.Location, &lead.Compensation, &postedAt, &lead.ContentHash, &state, &lead.CreatedAt, &lead.UpdatedAt,
		&score.ID, &score.LeadID, &score.Score, &scoreCategory, &score.Rationale, &score.DraftOpener, &score.ShouldNotify, &score.PromptVersion, &score.Model, &score.CreatedAt,
	)
	if err != nil {
		return err
	}
	if rawItemID.Valid {
		lead.RawItemID = rawItemID.Int64
	}
	if postedAt.Valid {
		lead.PostedAt = &postedAt.Time
	}
	lead.Category = core.Category(leadCategory)
	lead.State = core.LeadState(state)
	score.Category = core.Category(scoreCategory)
	return nil
}
