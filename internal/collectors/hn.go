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
	q := url.Values{}
	q.Set("query", `"seeking freelancer" OR "freelancer? seeking freelancer" OR "contract"`)
	q.Set("tags", "comment,story")
	q.Set("hitsPerPage", "50")
	endpoint := "https://hn.algolia.com/api/v1/search_by_date?" + q.Encode()

	req, _ := http.NewRequest(http.MethodGet, endpoint, nil)
	var res struct {
		Hits []map[string]any `json:"hits"`
	}
	if err := fetchJSON(ctx, h.client, req, &res); err != nil {
		return nil, err
	}

	items := make([]core.RawItem, 0, len(res.Hits))
	for _, hit := range res.Hits {
		payload, _ := json.Marshal(hit)
		id := stringField(hit, "objectID")
		link := firstString(stringField(hit, "url"), stringField(hit, "story_url"), fmt.Sprintf("https://news.ycombinator.com/item?id=%s", id))
		items = append(items, core.RawItem{
			Source:      h.Name(),
			ExternalID:  id,
			URL:         link,
			Payload:     payload,
			PublishedAt: parseAlgoliaTime(stringField(hit, "created_at")),
		})
	}
	return items, nil
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
