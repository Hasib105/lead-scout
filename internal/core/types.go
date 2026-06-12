package core

import (
	"context"
	"encoding/json"
	"time"
)

type Category string

const (
	CategoryGig     Category = "gig"
	CategoryFounder Category = "founder"
	CategoryJob     Category = "job"
)

type LeadState string

const (
	StateNew        LeadState = "new"
	StateSaved      LeadState = "saved"
	StateRejected   LeadState = "rejected"
	StateApproached LeadState = "approached"
	StateReplied    LeadState = "replied"
	StateCall       LeadState = "call"
	StateWon        LeadState = "won"
	StateLost       LeadState = "lost"
)

type RawItem struct {
	ID          int64
	Source      string
	ExternalID  string
	URL         string
	Payload     json.RawMessage
	FetchedAt   time.Time
	PublishedAt *time.Time
}

type Lead struct {
	ID           int64
	RawItemID    int64
	Source       string
	ExternalID   string
	Category     Category
	Title        string
	Body         string
	URL          string
	CanonicalURL string
	Author       string
	Company      string
	Location     string
	Compensation string
	PostedAt     *time.Time
	ContentHash  string
	State        LeadState
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type LeadScore struct {
	ID            int64
	LeadID        int64
	Score         int
	Category      Category
	Rationale     string
	DraftOpener   string
	ShouldNotify  bool
	PromptVersion string
	Model         string
	CreatedAt     time.Time
}

type LeadWithScore struct {
	Lead  Lead
	Score LeadScore
}

type Collector interface {
	Name() string
	Fetch(ctx context.Context) ([]RawItem, error)
}

type Normalizer interface {
	Normalize(raw RawItem) ([]Lead, error)
}

type Scorer interface {
	Score(ctx context.Context, lead Lead) (LeadScore, error)
}

type Notifier interface {
	SendHotLead(ctx context.Context, lead Lead, score LeadScore) error
	SendDigest(ctx context.Context, leads []LeadWithScore) error
}
