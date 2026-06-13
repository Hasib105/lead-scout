package collectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"lead-scout/internal/core"
)

type HN struct {
	client *http.Client
}

func NewHN(client *http.Client) HN {
	return HN{client: client}
}

func (h HN) Name() string { return "hn" }

func (h HN) Fetch(ctx context.Context) ([]core.RawItem, error) {
	queries := []string{
		`"looking for developer"`,
		`"need a developer"`,
		`"hire a developer"`,
		`"seeking freelancer"`,
		`"AI agent" contract`,
		`"agentic" contract`,
		`"fullstack" contract`,
		`"web developer" contract`,
	}

	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	seen := make(map[string]bool)
	var allItems []core.RawItem

	for _, query := range queries {
		q := url.Values{}
		q.Set("query", query)
		q.Set("tags", "story")
		q.Set("hitsPerPage", "20")
		q.Set("numericFilters", fmt.Sprintf("created_at_i>=%d", startOfMonth.Unix()))

		endpoint := "https://hn.algolia.com/api/v1/search_by_date?" + q.Encode()
		req, _ := http.NewRequest(http.MethodGet, endpoint, nil)
		var res struct {
			Hits []map[string]any `json:"hits"`
		}
		if err := fetchJSON(ctx, h.client, req, &res); err != nil {
			continue
		}

		for _, hit := range res.Hits {
			id := stringField(hit, "objectID")
			if seen[id] {
				continue
			}
			seen[id] = true
			payload, _ := json.Marshal(hit)
			link := firstString(stringField(hit, "url"), stringField(hit, "story_url"), fmt.Sprintf("https://news.ycombinator.com/item?id=%s", id))
			allItems = append(allItems, core.RawItem{
				Source:      h.Name(),
				ExternalID:  id,
				URL:         link,
				Payload:     payload,
				PublishedAt: parseAlgoliaTime(stringField(hit, "created_at")),
			})
		}
	}

	if len(allItems) > 50 {
		allItems = allItems[:50]
	}
	return allItems, nil
}

func parseAlgoliaTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}
