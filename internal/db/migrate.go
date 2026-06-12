package db

import (
	"context"
	"database/sql"
)

const schema = `
create table if not exists sources (
	name text primary key,
	kind text not null,
	enabled boolean not null default true,
	created_at timestamptz not null default now()
);

create table if not exists raw_items (
	id bigserial primary key,
	source text not null references sources(name),
	external_id text not null,
	url text not null,
	payload jsonb not null,
	fetched_at timestamptz not null default now(),
	published_at timestamptz,
	unique (source, external_id)
);

create table if not exists leads (
	id bigserial primary key,
	raw_item_id bigint references raw_items(id),
	source text not null references sources(name),
	external_id text not null,
	category text not null,
	title text not null,
	body text not null default '',
	url text not null,
	canonical_url text not null,
	author text not null default '',
	company text not null default '',
	location text not null default '',
	compensation text not null default '',
	posted_at timestamptz,
	content_hash text not null,
	state text not null default 'new',
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	unique (canonical_url),
	unique (source, external_id),
	unique (content_hash)
);

create table if not exists lead_scores (
	id bigserial primary key,
	lead_id bigint not null references leads(id) on delete cascade,
	score integer not null,
	category text not null,
	rationale text not null default '',
	draft_opener text not null default '',
	should_notify boolean not null default false,
	prompt_version text not null,
	model text not null,
	created_at timestamptz not null default now()
);

create index if not exists lead_scores_lead_id_created_at_idx on lead_scores (lead_id, created_at desc);
create index if not exists leads_state_category_created_at_idx on leads (state, category, created_at desc);

create table if not exists lead_events (
	id bigserial primary key,
	lead_id bigint not null references leads(id) on delete cascade,
	event_type text not null,
	note text not null default '',
	metadata jsonb not null default '{}',
	created_at timestamptz not null default now()
);

create table if not exists digests (
	id bigserial primary key,
	digest_type text not null,
	payload jsonb not null,
	sent_at timestamptz,
	created_at timestamptz not null default now()
);
`

var seedSources = []struct {
	name string
	kind string
}{
	{"hn", "api"},
	{"braintrust", "api"},
	{"remoteok", "api"},
	{"wwr", "rss"},
	{"reddit", "oauth"},
}

func Migrate(ctx context.Context, conn *sql.DB) error {
	if _, err := conn.ExecContext(ctx, schema); err != nil {
		return err
	}
	for _, src := range seedSources {
		if _, err := conn.ExecContext(ctx, `
			insert into sources (name, kind)
			values ($1, $2)
			on conflict (name) do update set kind = excluded.kind
		`, src.name, src.kind); err != nil {
			return err
		}
	}
	return nil
}
