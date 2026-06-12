package scoring

import (
	"context"
	"strings"

	"lead-scout/internal/core"
)

const HeuristicPromptVersion = "heuristic-v1"

type HeuristicScorer struct{}

func NewHeuristic() HeuristicScorer {
	return HeuristicScorer{}
}

func (h HeuristicScorer) Score(ctx context.Context, lead core.Lead) (core.LeadScore, error) {
	text := strings.ToLower(lead.Title + " " + lead.Body + " " + lead.Compensation + " " + lead.Location)
	score := 30

	score += hits(text, []string{"ai", "agent", "saas", "backend", "integration", "automation", "data", "postgres", "supabase"}, 5, 25)
	score += hits(text, []string{"contract", "freelance", "consultant", "project", "hourly", "$75", "$100", "$150", "budget"}, 6, 30)
	score += hits(text, []string{"urgent", "asap", "this week", "production", "launch", "blocked", "stuck"}, 5, 20)
	score += hits(text, []string{"lovable", "bolt", "vibe", "prototype", "mvp", "non-technical founder", "technical cofounder"}, 7, 28)

	if strings.Contains(text, "remote") || strings.Contains(text, "us") || strings.Contains(text, "europe") || strings.Contains(text, "eu") {
		score += 8
	}
	if strings.Contains(text, "full-time") && !strings.Contains(text, "contract") {
		score -= 12
	}
	if strings.Contains(text, "unpaid") || strings.Contains(text, "equity only") || strings.Contains(text, "intern") {
		score -= 30
	}
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	category := lead.Category
	if category == "" {
		category = inferCategory(text)
	}
	return core.LeadScore{
		LeadID:        lead.ID,
		Score:         score,
		Category:      category,
		Rationale:     rationale(score, category),
		DraftOpener:   "",
		ShouldNotify:  category == core.CategoryGig && score >= 80,
		PromptVersion: HeuristicPromptVersion,
		Model:         "heuristic",
	}, nil
}

func ShouldDeepScore(score core.LeadScore) bool {
	return score.Score >= 55
}

func hits(text string, keywords []string, each, max int) int {
	total := 0
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			total += each
		}
	}
	if total > max {
		return max
	}
	return total
}

func inferCategory(text string) core.Category {
	if strings.Contains(text, "lovable") || strings.Contains(text, "prototype") || strings.Contains(text, "founder") {
		return core.CategoryFounder
	}
	return core.CategoryGig
}

func rationale(score int, category core.Category) string {
	if score >= 80 {
		return "High-fit " + string(category) + " lead based on budget, urgency, and technical-fit signals."
	}
	if score >= 65 {
		return "Promising " + string(category) + " lead; include in digest for manual review."
	}
	if score >= 55 {
		return "Possible fit, worth deeper scoring before notifying."
	}
	return "Low-confidence fit from heuristic signals."
}
