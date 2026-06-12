package collectors

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"lead-scout/internal/config"
	"lead-scout/internal/core"
)

type Reddit struct {
	client *http.Client
	cfg    config.Config
}

func NewReddit(client *http.Client, cfg config.Config) Reddit {
	return Reddit{client: client, cfg: cfg}
}

func (r Reddit) Name() string { return "reddit" }

func (r Reddit) Fetch(ctx context.Context) ([]core.RawItem, error) {
	if !r.cfg.RedditConfigured() {
		return nil, ErrNotConfigured
	}
	token, err := r.token(ctx)
	if err != nil {
		return nil, err
	}

	queries := []struct {
		subreddit string
		query     string
	}{
		{"vibecoding", `"lovable" OR "bolt" OR "production" OR "supabase" OR "hire developer"`},
		{"lovable", `"production" OR "supabase" OR "developer" OR "stuck"`},
		{"nocode", `"hire developer" OR "production" OR "MVP" OR "supabase"`},
		{"startups", `"Looking For A Cofounder" OR "technical cofounder" OR "MVP"`},
	}

	var items []core.RawItem
	for _, q := range queries {
		got, err := r.search(ctx, token, q.subreddit, q.query)
		if err != nil {
			return nil, err
		}
		items = append(items, got...)
		time.Sleep(700 * time.Millisecond)
	}
	return items, nil
}

func (r Reddit) token(ctx context.Context) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://www.reddit.com/api/v1/access_token", strings.NewReader(form.Encode()))
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(r.cfg.RedditClientID+":"+r.cfg.RedditClientSecret)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", r.cfg.RedditUserAgent)

	var res struct {
		AccessToken string `json:"access_token"`
	}
	if err := fetchJSON(ctx, r.client, req, &res); err != nil {
		return "", err
	}
	return res.AccessToken, nil
}

func (r Reddit) search(ctx context.Context, token, subreddit, query string) ([]core.RawItem, error) {
	q := url.Values{}
	q.Set("q", query)
	q.Set("restrict_sr", "true")
	q.Set("sort", "new")
	q.Set("limit", "25")
	endpoint := "https://oauth.reddit.com/r/" + subreddit + "/search?" + q.Encode()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", r.cfg.RedditUserAgent)

	var res struct {
		Data struct {
			Children []struct {
				Data map[string]any `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := fetchJSON(ctx, r.client, req, &res); err != nil {
		return nil, err
	}

	items := make([]core.RawItem, 0, len(res.Data.Children))
	for _, child := range res.Data.Children {
		payload := child.Data
		payload["subreddit"] = subreddit
		body, _ := json.Marshal(payload)
		id := stringField(payload, "id")
		permalink := stringField(payload, "permalink")
		link := firstString(stringField(payload, "url"), "https://www.reddit.com"+permalink)
		items = append(items, core.RawItem{
			Source:      r.Name(),
			ExternalID:  subreddit + "_" + id,
			URL:         link,
			Payload:     body,
			PublishedAt: redditTime(payload["created_utc"]),
		})
	}
	return items, nil
}

func redditTime(v any) *time.Time {
	var seconds float64
	switch x := v.(type) {
	case float64:
		seconds = x
	case int64:
		seconds = float64(x)
	default:
		return nil
	}
	t := time.Unix(int64(seconds), 0)
	return &t
}
