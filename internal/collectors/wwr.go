package collectors

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"time"

	"lead-scout/internal/core"
)

type WWR struct {
	client *http.Client
}

func NewWWR(client *http.Client) WWR {
	return WWR{client: client}
}

func (w WWR) Name() string { return "wwr" }

func (w WWR) Fetch(ctx context.Context) ([]core.RawItem, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://weworkremotely.com/categories/remote-programming-jobs.rss", nil)
	req.Header.Set("User-Agent", "lead-scout/0.1")
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, errStatus(resp.Status)
	}

	var feed rssFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, err
	}

	items := make([]core.RawItem, 0, len(feed.Channel.Items))
	for _, item := range feed.Channel.Items {
		payload, _ := json.Marshal(map[string]string{
			"title":       item.Title,
			"url":         item.Link,
			"description": item.Description,
			"date":        item.PubDate,
		})
		items = append(items, core.RawItem{
			Source:      w.Name(),
			ExternalID:  firstString(item.GUID, item.Link),
			URL:         item.Link,
			Payload:     payload,
			PublishedAt: parseRSSTime(item.PubDate),
		})
	}
	return items, nil
}

type rssFeed struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

func parseRSSTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}
