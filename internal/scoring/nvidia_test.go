package scoring

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"lead-scout/internal/core"
)

func TestNVIDIAScorerSendsNonStreamingJSONRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("authorization header = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Fatalf("accept header = %q", r.Header.Get("Accept"))
		}

		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req["stream"] != false {
			t.Fatalf("stream = %#v, want false", req["stream"])
		}
		if req["model"] != "google/gemma-4-31b-it" {
			t.Fatalf("model = %#v", req["model"])
		}

		_, _ = w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "{\"score\":88,\"category\":\"gig\",\"rationale\":\"Strong fit\",\"draft_opener\":\"Hi\",\"should_notify\":true}"
				}
			}]
		}`))
	}))
	defer server.Close()

	score, err := NewNVIDIA("test-key", server.URL, "google/gemma-4-31b-it", NewHeuristic()).Score(context.Background(), core.Lead{
		ID:       7,
		Category: core.CategoryGig,
		Title:    "AI SaaS contract",
		Body:     "$100/hr remote urgent backend project",
	})
	if err != nil {
		t.Fatal(err)
	}
	if score.Score != 88 || !score.ShouldNotify || score.Model != "nvidia:google/gemma-4-31b-it" {
		t.Fatalf("score = %+v", score)
	}
}

func TestNVIDIAScorerFallsBackWithoutKey(t *testing.T) {
	score, err := NewNVIDIA("", "", "", NewHeuristic()).Score(context.Background(), core.Lead{
		ID:       9,
		Category: core.CategoryGig,
		Title:    "Senior developer needed",
		Body:     "Contract work for backend API development",
	})
	if err != nil {
		t.Fatal(err)
	}
	if score.Model != "heuristic" {
		t.Fatalf("model = %q, want heuristic", score.Model)
	}
}
