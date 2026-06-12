package collectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"lead-scout/internal/core"
)

type RemoteOK struct {
	client *http.Client
}

func NewRemoteOK(client *http.Client) RemoteOK {
	return RemoteOK{client: client}
}

func (r RemoteOK) Name() string { return "remoteok" }

func (r RemoteOK) Fetch(ctx context.Context) ([]core.RawItem, error) {
	req, _ := http.NewRequest(http.MethodGet, "https://remoteok.com/api", nil)
	req.Header.Set("User-Agent", "lead-scout/0.1")

	var rows []map[string]any
	if err := fetchJSON(ctx, r.client, req, &rows); err != nil {
		return nil, err
	}

	items := make([]core.RawItem, 0, len(rows))
	for _, row := range rows {
		if _, hasLegal := row["legal"]; hasLegal {
			continue
		}
		payload, _ := json.Marshal(row)
		id := firstString(stringField(row, "id"), stringField(row, "slug"))
		if id == "" {
			id = fmt.Sprint(row["epoch"])
		}
		link := firstString(stringField(row, "url"), "https://remoteok.com/remote-jobs/"+stringField(row, "slug"))
		items = append(items, core.RawItem{
			Source:     r.Name(),
			ExternalID: id,
			URL:        link,
			Payload:    payload,
		})
	}
	return items, nil
}
