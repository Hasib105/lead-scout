package collectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"lead-scout/internal/core"
)

type Braintrust struct {
	client *http.Client
}

func NewBraintrust(client *http.Client) Braintrust {
	return Braintrust{client: client}
}

func (b Braintrust) Name() string { return "braintrust" }

func (b Braintrust) Fetch(ctx context.Context) ([]core.RawItem, error) {
	req, _ := http.NewRequest(http.MethodGet, "https://app.usebraintrust.com/api/jobs/", nil)
	var payload any
	if err := fetchJSON(ctx, b.client, req, &payload); err != nil {
		return nil, err
	}

	jobs := unwrapList(payload)
	items := make([]core.RawItem, 0, len(jobs))
	for i, job := range jobs {
		m, ok := job.(map[string]any)
		if !ok {
			continue
		}
		body, _ := json.Marshal(m)
		id := firstString(stringField(m, "id"), stringField(m, "uuid"), fmt.Sprintf("braintrust-%d", i))
		link := firstString(stringField(m, "url"), stringField(m, "apply_url"), "https://app.usebraintrust.com/jobs")
		items = append(items, core.RawItem{
			Source:     b.Name(),
			ExternalID: id,
			URL:        link,
			Payload:    body,
		})
	}
	return items, nil
}
