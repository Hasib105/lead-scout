package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lead-scout/internal/core"
)

type Client struct {
	token  string
	chatID string
	http   *http.Client
}

func New(token, chatID string) Client {
	return Client{
		token:  token,
		chatID: chatID,
		http:   &http.Client{Timeout: 20 * time.Second},
	}
}

func (c Client) Configured() bool {
	return c.token != "" && c.chatID != ""
}

func (c Client) SendHotLead(ctx context.Context, lead core.Lead, score core.LeadScore) error {
	text := fmt.Sprintf("<b>Hot lead: %s</b>\nScore: %d\nSource: %s\n%s\n\n%s",
		escape(lead.Title), score.Score, escape(lead.Source), escape(lead.URL), escape(score.Rationale))
	return c.send(ctx, text, keyboard(lead.ID))
}

func (c Client) SendDigest(ctx context.Context, leads []core.LeadWithScore) error {
	if len(leads) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("<b>Daily founder digest</b>\n")
	for i, item := range leads {
		fmt.Fprintf(&b, "\n%d. <b>%s</b>\nScore: %d Source: %s\n%s\n%s\n",
			i+1, escape(item.Lead.Title), item.Score.Score, escape(item.Lead.Source), escape(item.Lead.URL), escape(item.Score.Rationale))
	}
	return c.send(ctx, b.String(), nil)
}

func (c Client) SendTest(ctx context.Context) error {
	return c.send(ctx, "Lead Scout Telegram test", nil)
}

func (c Client) send(ctx context.Context, text string, replyMarkup any) error {
	if !c.Configured() {
		return errors.New("telegram is not configured")
	}
	payload := map[string]any{
		"chat_id":                  c.chatID,
		"text":                     text,
		"parse_mode":               "HTML",
		"disable_web_page_preview": false,
	}
	if replyMarkup != nil {
		payload["reply_markup"] = replyMarkup
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("sendMessage"), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("telegram send failed: %s", resp.Status)
	}
	return nil
}

func (c Client) GetUpdates(ctx context.Context, offset int64) ([]Update, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint("getUpdates"), nil)
	q := req.URL.Query()
	q.Set("timeout", "25")
	if offset > 0 {
		q.Set("offset", strconv.FormatInt(offset, 10))
	}
	req.URL.RawQuery = q.Encode()
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("telegram updates failed: %s", resp.Status)
	}
	var out struct {
		OK     bool     `json:"ok"`
		Result []Update `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Result, nil
}

func (c Client) AnswerCallback(ctx context.Context, callbackID, text string) error {
	payload := map[string]any{"callback_query_id": callbackID, "text": text}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("answerCallbackQuery"), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("telegram callback answer failed: %s", resp.Status)
	}
	return nil
}

func (c Client) endpoint(method string) string {
	return "https://api.telegram.org/bot" + c.token + "/" + method
}

type Update struct {
	UpdateID      int64          `json:"update_id"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

type CallbackQuery struct {
	ID   string `json:"id"`
	Data string `json:"data"`
}

type Action struct {
	LeadID int64
	State  core.LeadState
}

func ParseAction(data string) (Action, error) {
	parts := strings.Split(data, ":")
	if len(parts) != 3 || parts[0] != "lead" {
		return Action{}, errors.New("invalid callback data")
	}
	leadID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return Action{}, err
	}
	state := core.LeadState(parts[2])
	switch state {
	case core.StateSaved, core.StateRejected, core.StateApproached, core.StateReplied, core.StateCall, core.StateWon, core.StateLost:
		return Action{LeadID: leadID, State: state}, nil
	default:
		return Action{}, errors.New("invalid callback state")
	}
}

func keyboard(leadID int64) map[string]any {
	row := func(states ...core.LeadState) []map[string]string {
		buttons := make([]map[string]string, 0, len(states))
		for _, state := range states {
			buttons = append(buttons, map[string]string{
				"text":          buttonText(state),
				"callback_data": fmt.Sprintf("lead:%d:%s", leadID, state),
			})
		}
		return buttons
	}
	return map[string]any{
		"inline_keyboard": [][]map[string]string{
			row(core.StateSaved, core.StateRejected),
			row(core.StateApproached, core.StateReplied, core.StateCall),
			row(core.StateWon, core.StateLost),
		},
	}
}

func buttonText(state core.LeadState) string {
	switch state {
	case core.StateSaved:
		return "Save"
	case core.StateRejected:
		return "Reject"
	case core.StateApproached:
		return "Approached"
	case core.StateReplied:
		return "Replied"
	case core.StateCall:
		return "Call"
	case core.StateWon:
		return "Won"
	case core.StateLost:
		return "Lost"
	default:
		return string(state)
	}
}

func escape(s string) string {
	return html.EscapeString(strings.TrimSpace(s))
}
