package scoring

import (
	"context"
	"strings"

	"lead-scout/internal/core"
)

const HeuristicPromptVersion = "heuristic-v2"

type HeuristicScorer struct{}

func NewHeuristic() HeuristicScorer {
	return HeuristicScorer{}
}

func (h HeuristicScorer) Score(ctx context.Context, lead core.Lead) (core.LeadScore, error) {
	text := strings.ToLower(lead.Title + " " + lead.Body + " " + lead.Compensation + " " + lead.Location)
	score := 30

	// Hard skip - reject immediately, don't waste AI calls
	if shouldSkip(text) {
		return core.LeadScore{
			LeadID:        lead.ID,
			Score:         0,
			Category:      lead.Category,
			Rationale:     "Rejected: low-quality signal (job board repost, recruiter spam, or irrelevant content).",
			DraftOpener:   "",
			ShouldNotify:  false,
			PromptVersion: HeuristicPromptVersion,
			Model:         "heuristic-skip",
		}, nil
	}

	// Tech fit signals - higher weight for your specialties
	score += hits(text, []string{"ai", "agent", "agentic", "llm", "gpt", "openai", "langchain", "vector", "rag"}, 7, 35)
	score += hits(text, []string{"web", "fullstack", "full-stack", "frontend", "backend", "react", "nextjs", "node", "typescript"}, 6, 30)
	score += hits(text, []string{"saas", "integration", "automation", "api", "database", "postgres", "supabase"}, 5, 25)
	
	// Budget/compensation signals
	score += hits(text, []string{"contract", "freelance", "consultant", "project", "hourly", "$75", "$100", "$150", "$200", "budget", "paid"}, 5, 25)
	
	// Urgency signals
	score += hits(text, []string{"urgent", "asap", "this week", "production", "launch", "blocked", "stuck", "broken", "need help"}, 5, 20)
	
	// Founder/vibe-code signals
	score += hits(text, []string{"lovable", "bolt", "vibe", "prototype", "mvp", "non-technical founder", "technical cofounder", "no-code"}, 6, 24)

	// Location bonus
	if strings.Contains(text, "remote") || strings.Contains(text, "us") || strings.Contains(text, "europe") || strings.Contains(text, "eu") {
		score += 6
	}
	
	// Penalty for full-time (not what we want)
	if strings.Contains(text, "full-time") && !strings.Contains(text, "contract") {
		score -= 15
	}
	
	// Hard penalties
	if strings.Contains(text, "unpaid") || strings.Contains(text, "equity only") || strings.Contains(text, "intern") {
		score -= 30
	}
	if strings.Contains(text, "senior") || strings.Contains(text, "experienced") {
		score += 5
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

// shouldSkip returns true for leads that are clearly noise and should not reach AI
func shouldSkip(text string) bool {
	// Job board reposts
	skipPhrases := []string{
		"we're hiring across",
		"open positions",
		"view all jobs",
		"apply now on",
		"job board",
		"click here to apply",
		"share this job",
		"looking for work",
		"seeking work",
		"available for hire",
		"my resume",
		"my portfolio",
	}
	for _, phrase := range skipPhrases {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	
	// Too short to be meaningful
	if len(strings.TrimSpace(text)) < 50 {
		return true
	}
	
	return false
}

// ShouldDeepScore determines if a lead needs AI scoring
// Leads below 40 are clearly low-fit, skip AI
// Leads 40-54 get heuristic only (borderline)
// Leads 55+ get AI scoring for precision
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
