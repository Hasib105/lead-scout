package normalize

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"

	"lead-scout/internal/core"
)

var tags = regexp.MustCompile(`<[^>]+>`)

type Normalizer struct{}

func New() Normalizer {
	return Normalizer{}
}

func (n Normalizer) Normalize(raw core.RawItem) ([]core.Lead, error) {
	switch raw.Source {
	case "hn":
		return n.hn(raw)
	case "braintrust":
		return n.braintrust(raw)
	case "remoteok":
		return n.remoteOK(raw)
	case "wwr":
		return n.wwr(raw)
	case "reddit":
		return n.reddit(raw)
	default:
		return nil, nil
	}
}

type genericPayload struct {
	ID           any    `json:"id"`
	ObjectID     string `json:"objectID"`
	Title        string `json:"title"`
	StoryTitle   string `json:"story_title"`
	Name         string `json:"name"`
	Company      string `json:"company"`
	Author       string `json:"author"`
	By           string `json:"by"`
	URL          string `json:"url"`
	StoryURL     string `json:"story_url"`
	Text         string `json:"text"`
	CommentText  string `json:"comment_text"`
	Description  string `json:"description"`
	Body         string `json:"body"`
	Location     string `json:"location"`
	Compensation string `json:"compensation"`
	Salary       string `json:"salary"`
	Position     string `json:"position"`
	Category     string `json:"category"`
	Subreddit    string `json:"subreddit"`
	CreatedUTC   any    `json:"created_utc"`
	Date         string `json:"date"`
}

func (n Normalizer) hn(raw core.RawItem) ([]core.Lead, error) {
	var p genericPayload
	if err := json.Unmarshal(raw.Payload, &p); err != nil {
		return nil, err
	}
	title := first(p.Title, p.StoryTitle, "HN lead")
	body := clean(first(p.CommentText, p.Text, p.Description))
	link := first(p.URL, p.StoryURL, raw.URL)
	category := core.CategoryGig
	if founderish(title + " " + body) {
		category = core.CategoryFounder
	}
	return []core.Lead{buildLead(raw, category, title, body, link, first(p.Author, p.By), p.Company, p.Location, first(p.Compensation, p.Salary), raw.PublishedAt)}, nil
}

func (n Normalizer) braintrust(raw core.RawItem) ([]core.Lead, error) {
	var p genericPayload
	if err := json.Unmarshal(raw.Payload, &p); err != nil {
		return nil, err
	}
	title := first(p.Title, p.Name, "Braintrust role")
	body := clean(first(p.Description, p.Body, p.Text))
	comp := first(p.Compensation, p.Salary)
	return []core.Lead{buildLead(raw, core.CategoryGig, title, body, first(p.URL, raw.URL), p.Author, p.Company, p.Location, comp, raw.PublishedAt)}, nil
}

func (n Normalizer) remoteOK(raw core.RawItem) ([]core.Lead, error) {
	var p genericPayload
	if err := json.Unmarshal(raw.Payload, &p); err != nil {
		return nil, err
	}
	title := first(p.Position, p.Title, "RemoteOK role")
	body := clean(first(p.Description, p.Body, p.Text))
	return []core.Lead{buildLead(raw, core.CategoryGig, title, body, first(p.URL, raw.URL), p.Author, p.Company, p.Location, first(p.Compensation, p.Salary), raw.PublishedAt)}, nil
}

func (n Normalizer) wwr(raw core.RawItem) ([]core.Lead, error) {
	var p genericPayload
	if err := json.Unmarshal(raw.Payload, &p); err != nil {
		return nil, err
	}
	title := first(p.Title, "WWR role")
	body := clean(first(p.Description, p.Body))
	return []core.Lead{buildLead(raw, core.CategoryGig, title, body, first(p.URL, raw.URL), p.Author, p.Company, p.Location, first(p.Compensation, p.Salary), raw.PublishedAt)}, nil
}

func (n Normalizer) reddit(raw core.RawItem) ([]core.Lead, error) {
	var p genericPayload
	if err := json.Unmarshal(raw.Payload, &p); err != nil {
		return nil, err
	}
	title := first(p.Title, "Reddit founder lead")
	body := clean(first(p.Body, p.Text, p.Description))
	category := core.CategoryFounder
	if strings.Contains(strings.ToLower(title+" "+body), "hiring") {
		category = core.CategoryGig
	}
	return []core.Lead{buildLead(raw, category, title, body, first(p.URL, raw.URL), first(p.Author, p.By), p.Company, p.Location, first(p.Compensation, p.Salary), raw.PublishedAt)}, nil
}

func buildLead(raw core.RawItem, category core.Category, title, body, link, author, company, location, compensation string, postedAt *time.Time) core.Lead {
	title = clean(title)
	body = clean(body)
	link = strings.TrimSpace(link)
	if link == "" {
		link = raw.URL
	}
	lead := core.Lead{
		RawItemID:    raw.ID,
		Source:       raw.Source,
		ExternalID:   raw.ExternalID,
		Category:     category,
		Title:        title,
		Body:         body,
		URL:          link,
		CanonicalURL: CanonicalURL(link),
		Author:       clean(author),
		Company:      clean(company),
		Location:     clean(location),
		Compensation: clean(compensation),
		PostedAt:     postedAt,
		State:        core.StateNew,
	}
	lead.ContentHash = ContentHash(lead.Title, lead.Body, lead.CanonicalURL)
	return lead
}

func CanonicalURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Fragment = ""
	q := u.Query()
	for key := range q {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") || lower == "ref" || lower == "source" {
			q.Del(key)
		}
	}
	u.RawQuery = q.Encode()
	u.Host = strings.ToLower(u.Host)
	if u.Path != "/" {
		u.Path = strings.TrimRight(u.Path, "/")
	}
	return strings.TrimRight(u.String(), "/")
}

func ContentHash(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte(strings.ToLower(strings.TrimSpace(part))))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func clean(s string) string {
	s = html.UnescapeString(s)
	s = tags.ReplaceAllString(s, " ")
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

func first(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func founderish(s string) bool {
	s = strings.ToLower(s)
	keywords := []string{"lovable", "bolt", "vibecode", "vibe code", "prototype", "supabase", "rls", "production", "mvp", "founder"}
	for _, keyword := range keywords {
		if strings.Contains(s, keyword) {
			return true
		}
	}
	return false
}
