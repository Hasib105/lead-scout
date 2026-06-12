package collectors

import (
	"context"
	"errors"
	"net/http"
	"time"

	"lead-scout/internal/config"
	"lead-scout/internal/core"
)

var ErrNotConfigured = errors.New("collector is not configured")

func Registry(cfg config.Config) map[string]core.Collector {
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	return map[string]core.Collector{
		"hn":         NewHN(client),
		"braintrust": NewBraintrust(client),
		"remoteok":   NewRemoteOK(client),
		"wwr":        NewWWR(client),
		"reddit":     NewReddit(client, cfg),
	}
}

func fetchJSON(ctx context.Context, client *http.Client, req *http.Request, out any) error {
	req = req.WithContext(ctx)
	req.Header.Set("Accept", "application/json")
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "lead-scout/0.1")
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return errors.New(resp.Status)
	}
	return jsonDecoder(resp.Body, out)
}

func ptrTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
