package scoring

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"lead-scout/internal/core"
)

const NVIDIAAPIPromptVersion = "lead-score-nvidia-v1"

type NVIDIAScorer struct {
	APIKey  string
	BaseURL string
	Model   string
	Client  *http.Client
	Backup  core.Scorer
}

func NewNVIDIA(apiKey, baseURL, model string, backup core.Scorer) NVIDIAScorer {
	if baseURL == "" {
		baseURL = "https://integrate.api.nvidia.com/v1/chat/completions"
	}
	if model == "" {
		model = "google/gemma-4-31b-it"
	}
	return NVIDIAScorer{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
		Client:  &http.Client{Timeout: 90 * time.Second},
		Backup:  backup,
	}
}

func (s NVIDIAScorer) Score(ctx context.Context, lead core.Lead) (core.LeadScore, error) {
	if strings.TrimSpace(s.APIKey) == "" {
		return s.Backup.Score(ctx, lead)
	}

	prompt, err := buildPrompt(lead)
	if err != nil {
		return s.Backup.Score(ctx, lead)
	}

	payload := map[string]any{
		"model": s.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens":  16384,
		"temperature": 1.00,
		"top_p":       0.95,
		"stream":      false,
		"chat_template_kwargs": map[string]bool{
			"enable_thinking": true,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return s.Backup.Score(ctx, lead)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.BaseURL, bytes.NewReader(body))
	if err != nil {
		return s.Backup.Score(ctx, lead)
	}
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fallbackWithNote(ctx, s.Backup, lead, "NVIDIA API unavailable; used heuristic fallback.", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fallbackWithNote(ctx, s.Backup, lead, fmt.Sprintf("NVIDIA API returned %s; used heuristic fallback.", resp.Status), errors.New(string(respBody)))
	}

	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&completion); err != nil {
		return fallbackWithNote(ctx, s.Backup, lead, "NVIDIA API returned invalid response JSON; used heuristic fallback.", err)
	}
	if len(completion.Choices) == 0 || strings.TrimSpace(completion.Choices[0].Message.Content) == "" {
		return fallbackWithNote(ctx, s.Backup, lead, "NVIDIA API returned no content; used heuristic fallback.", errors.New("empty choices"))
	}

	var res struct {
		Score        int    `json:"score"`
		Category     string `json:"category"`
		Rationale    string `json:"rationale"`
		DraftOpener  string `json:"draft_opener"`
		ShouldNotify bool   `json:"should_notify"`
	}
	if err := json.Unmarshal(extractJSON(completion.Choices[0].Message.Content), &res); err != nil {
		return fallbackWithNote(ctx, s.Backup, lead, "NVIDIA API returned invalid scoring JSON; used heuristic fallback.", err)
	}

	if res.Score < 0 {
		res.Score = 0
	}
	if res.Score > 100 {
		res.Score = 100
	}
	category := core.Category(res.Category)
	if category == "" {
		category = lead.Category
	}
	return core.LeadScore{
		LeadID:        lead.ID,
		Score:         res.Score,
		Category:      category,
		Rationale:     res.Rationale,
		DraftOpener:   res.DraftOpener,
		ShouldNotify:  res.ShouldNotify || (category == core.CategoryGig && res.Score >= 80),
		PromptVersion: NVIDIAAPIPromptVersion,
		Model:         "nvidia:" + s.Model,
	}, nil
}

func fallbackWithNote(ctx context.Context, backup core.Scorer, lead core.Lead, note string, cause error) (core.LeadScore, error) {
	score, fallbackErr := backup.Score(ctx, lead)
	if fallbackErr != nil {
		return score, errors.Join(cause, fallbackErr)
	}
	score.Rationale = score.Rationale + " " + note
	return score, nil
}

func buildPrompt(lead core.Lead) (string, error) {
	// Truncate body to save tokens - only need first 500 chars for scoring
	body := lead.Body
	if len(body) > 500 {
		body = body[:500] + "..."
	}
	
	// Only send relevant fields
	payload, err := json.MarshalIndent(map[string]any{
		"title":        lead.Title,
		"body":         body,
		"source":       lead.Source,
		"category":     lead.Category,
		"location":     lead.Location,
		"compensation": lead.Compensation,
	}, "", "  ")
	if err != nil {
		return "", err
	}
	return `Score this lead for a solo senior engineer targeting premium remote US/EU freelance gigs and early-stage founder prototype rescue work.

Return only strict JSON with this shape:
{"score":82,"category":"gig","rationale":"...","draft_opener":"...","should_notify":true}

Rules:
- Score 80+ only for high-fit leads with budget/urgency/technical-fit signals.
- Founder leads are usually digest candidates, not immediate alerts.
- Never suggest automated outreach; the opener is a manual draft for the user to edit.
- Penalize unpaid, equity-only, junior, internship, and generic full-time roles.
- Keep rationale under 50 words.

Lead:
` + string(payload), nil
}

func extractJSON(s string) []byte {
	s = strings.TrimSpace(s)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end >= start {
		return []byte(s[start : end+1])
	}
	return []byte(s)
}
