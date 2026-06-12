package normalize

import (
	"encoding/json"
	"testing"

	"lead-scout/internal/core"
)

func TestCanonicalURLStripsTracking(t *testing.T) {
	got := CanonicalURL("https://Example.com/path/?utm_source=x&keep=1#frag")
	want := "https://example.com/path?keep=1"
	if got != want {
		t.Fatalf("canonical url = %q, want %q", got, want)
	}
}

func TestHNNormalizeFounderSignal(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"objectID":     "1",
		"title":        "Need help taking Lovable prototype to production",
		"comment_text": "Supabase RLS is blocking launch",
		"url":          "https://example.com?utm_campaign=x",
		"author":       "founder",
	})
	n := New()
	leads, err := n.Normalize(core.RawItem{
		Source:     "hn",
		ExternalID: "1",
		URL:        "https://example.com",
		Payload:    payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(leads) != 1 {
		t.Fatalf("got %d leads", len(leads))
	}
	if leads[0].Category != core.CategoryFounder {
		t.Fatalf("category = %q", leads[0].Category)
	}
	if leads[0].CanonicalURL != "https://example.com" {
		t.Fatalf("canonical = %q", leads[0].CanonicalURL)
	}
}
