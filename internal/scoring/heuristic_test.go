package scoring

import (
	"context"
	"testing"

	"lead-scout/internal/core"
)

func TestHeuristicScoresPremiumGigHot(t *testing.T) {
	score, err := NewHeuristic().Score(context.Background(), core.Lead{
		ID:           42,
		Category:     core.CategoryGig,
		Title:        "Need AI SaaS backend contractor this week",
		Body:         "Remote US/EU, $100/hr budget, production launch is blocked by Supabase integration.",
		Compensation: "$100/hr",
		Location:     "Remote US/EU",
	})
	if err != nil {
		t.Fatal(err)
	}
	if score.Score < 80 {
		t.Fatalf("score = %d, want hot", score.Score)
	}
	if !score.ShouldNotify {
		t.Fatal("expected hot gig notification")
	}
}

func TestHeuristicPenalizesUnpaid(t *testing.T) {
	score, err := NewHeuristic().Score(context.Background(), core.Lead{
		Category: core.CategoryGig,
		Title:    "AI intern wanted",
		Body:     "Unpaid equity only full-time internship.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if score.Score >= 55 {
		t.Fatalf("score = %d, want below deep-score threshold", score.Score)
	}
}
