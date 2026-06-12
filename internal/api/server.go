package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lead-scout/internal/collectors"
	"lead-scout/internal/config"
	"lead-scout/internal/core"
	"lead-scout/internal/db"
	"lead-scout/internal/normalize"
	"lead-scout/internal/scoring"
	"lead-scout/internal/telegram"
)

type Server struct {
	cfg  config.Config
	repo *db.Repository
	mux  *http.ServeMux
}

func NewServer(cfg config.Config, repo *db.Repository) *Server {
	s := &Server{
		cfg:  cfg,
		repo: repo,
		mux:  http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /", s.redirectDocs)
	s.mux.HandleFunc("GET /docs", s.docs)
	s.mux.HandleFunc("GET /openapi.json", s.openapi)
	s.mux.HandleFunc("GET /health", s.health)
	s.mux.HandleFunc("POST /api/collect", s.collect)
	s.mux.HandleFunc("POST /api/score", s.score)
	s.mux.HandleFunc("POST /api/digest", s.digest)
	s.mux.HandleFunc("POST /api/telegram/test", s.telegramTest)
	s.mux.HandleFunc("POST /api/telegram/lead-test", s.telegramLeadTest)
	s.mux.HandleFunc("GET /api/leads", s.listLeads)
	s.mux.HandleFunc("PATCH /api/leads/{id}/state", s.updateLeadState)
}

func (s *Server) redirectDocs(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/docs", http.StatusFound)
}

func (s *Server) docs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html>
<html>
  <head>
    <title>Lead Scout API Docs</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
  </head>
  <body>
    <div id="app"></div>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
    <script>
      Scalar.createApiReference('#app', {
        url: '/openapi.json',
        theme: 'default',
        layout: 'modern'
      })
    </script>
  </body>
</html>`))
}

func (s *Server) openapi(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(openAPIJSON))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "lead-scout"})
}

func (s *Server) collect(w http.ResponseWriter, r *http.Request) {
	var req collectRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !req.All && req.Source == "" {
		writeError(w, http.StatusBadRequest, errors.New("source or all is required"))
		return
	}

	registry := collectors.Registry(s.cfg)
	names := []string{req.Source}
	if req.All {
		names = []string{"hn", "braintrust", "remoteok", "wwr", "reddit"}
	}

	normalizer := normalize.New()
	res := collectResponse{Sources: make([]sourceCollectResult, 0, len(names))}
	for _, name := range names {
		collector, ok := registry[name]
		if !ok {
			writeError(w, http.StatusBadRequest, fmt.Errorf("unknown source %q", name))
			return
		}
		result := sourceCollectResult{Source: name}
		rawItems, err := collector.Fetch(r.Context())
		if err != nil {
			if errors.Is(err, collectors.ErrNotConfigured) && req.All {
				result.Skipped = true
				result.Error = "not configured"
				res.Sources = append(res.Sources, result)
				continue
			}
			writeError(w, http.StatusBadGateway, fmt.Errorf("collect %s: %w", name, err))
			return
		}
		result.Fetched = len(rawItems)
		for _, raw := range rawItems {
			savedRaw, err := s.repo.UpsertRawItem(r.Context(), raw)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			leads, err := normalizer.Normalize(savedRaw)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			for _, lead := range leads {
				if lead.Title == "" {
					continue
				}
				if _, err := s.repo.UpsertLead(r.Context(), lead); err != nil {
					writeError(w, http.StatusInternalServerError, err)
					return
				}
				result.Normalized++
			}
		}
		res.Sources = append(res.Sources, result)
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) score(w http.ResponseWriter, r *http.Request) {
	req := scoreRequest{Since: "24h"}
	if r.Body != nil && r.ContentLength != 0 {
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	since, err := time.ParseDuration(req.Since)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	leads, err := s.repo.PendingScoreLeads(r.Context(), since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	heuristic := scoring.NewHeuristic()
	aiScorer := scoring.NewNVIDIA(s.cfg.NVIDIAAPIKey, s.cfg.NVIDIABaseURL, s.cfg.NVIDIAModel, heuristic)
	notifier := telegram.New(s.cfg.TelegramBotToken, s.cfg.TelegramChatID)
	scored := 0
	for _, lead := range leads {
		hScore, err := heuristic.Score(r.Context(), lead)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		finalScore := hScore
		if scoring.ShouldDeepScore(hScore) {
			finalScore, err = aiScorer.Score(r.Context(), lead)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
		}
		finalScore.LeadID = lead.ID
		saved, err := s.repo.InsertLeadScore(r.Context(), finalScore)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if notifier.Configured() && lead.Category == core.CategoryGig && saved.ShouldNotify && saved.Score >= 80 {
			if err := notifier.SendHotLead(r.Context(), lead, saved); err != nil {
				writeError(w, http.StatusBadGateway, err)
				return
			}
			if err := s.repo.RecordEvent(r.Context(), lead.ID, "notified", "hot gig alert sent", map[string]string{"channel": "telegram"}); err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
		}
		scored++
	}
	writeJSON(w, http.StatusOK, scoreResponse{Scored: scored})
}

func (s *Server) digest(w http.ResponseWriter, r *http.Request) {
	req := digestRequest{Limit: 20}
	if r.Body != nil && r.ContentLength != 0 {
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 20
	}
	candidates, err := s.repo.DigestCandidates(r.Context(), core.CategoryFounder, 65, req.Limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if req.Send {
		if len(candidates) == 0 {
			writeJSON(w, http.StatusOK, digestResponse{Sent: false, Leads: []apiLeadWithScore{}, Message: "no qualifying founder leads to send"})
			return
		}
		notifier := telegram.New(s.cfg.TelegramBotToken, s.cfg.TelegramChatID)
		if !notifier.Configured() {
			writeError(w, http.StatusBadRequest, errors.New("telegram is not configured"))
			return
		}
		if err := notifier.SendDigest(r.Context(), candidates); err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		for _, item := range candidates {
			if err := s.repo.RecordEvent(r.Context(), item.Lead.ID, "digest_sent", "daily founder digest", map[string]string{"channel": "telegram"}); err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
		}
	}
	writeJSON(w, http.StatusOK, digestResponse{Sent: req.Send && len(candidates) > 0, Leads: toAPILeadWithScores(candidates)})
}

func (s *Server) telegramTest(w http.ResponseWriter, r *http.Request) {
	notifier := telegram.New(s.cfg.TelegramBotToken, s.cfg.TelegramChatID)
	if !notifier.Configured() {
		writeError(w, http.StatusBadRequest, errors.New("telegram is not configured"))
		return
	}
	if err := notifier.SendTest(r.Context()); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sent": true})
}

func (s *Server) telegramLeadTest(w http.ResponseWriter, r *http.Request) {
	var req apiLeadWithScore
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Lead.Title == "" || req.Lead.URL == "" {
		writeError(w, http.StatusBadRequest, errors.New("lead.title and lead.url are required"))
		return
	}
	if req.Score.Score == 0 {
		req.Score.Score = 80
	}
	if req.Score.Rationale == "" {
		req.Score.Rationale = "Manual Scalar test lead."
	}
	notifier := telegram.New(s.cfg.TelegramBotToken, s.cfg.TelegramChatID)
	if !notifier.Configured() {
		writeError(w, http.StatusBadRequest, errors.New("telegram is not configured"))
		return
	}
	if err := notifier.SendHotLead(r.Context(), fromAPILead(req.Lead), fromAPILeadScore(req.Score)); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sent": true})
}

func (s *Server) listLeads(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		limit = n
	}
	leads, err := s.repo.ListLeads(
		r.Context(),
		core.Category(r.URL.Query().Get("category")),
		core.LeadState(r.URL.Query().Get("state")),
		limit,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, leadListResponse{Leads: toAPILeadWithScores(leads)})
}

func (s *Server) updateLeadState(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var req updateStateRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.repo.SetLeadState(r.Context(), id, req.State, req.Note); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, updateStateResponse{ID: id, State: req.State, OK: true})
}

func ListenAndServe(ctx context.Context, cfg config.Config, repo *db.Repository, addr string) error {
	if strings.TrimSpace(addr) == "" {
		addr = cfg.APIAddr
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           NewServer(cfg, repo).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		return ctx.Err()
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

type collectRequest struct {
	Source string `json:"source"`
	All    bool   `json:"all"`
}

type collectResponse struct {
	Sources []sourceCollectResult `json:"sources"`
}

type sourceCollectResult struct {
	Source     string `json:"source"`
	Fetched    int    `json:"fetched"`
	Normalized int    `json:"normalized"`
	Skipped    bool   `json:"skipped"`
	Error      string `json:"error,omitempty"`
}

type scoreRequest struct {
	Since string `json:"since"`
}

type scoreResponse struct {
	Scored int `json:"scored"`
}

type digestRequest struct {
	Send  bool `json:"send"`
	Limit int  `json:"limit"`
}

type digestResponse struct {
	Sent    bool               `json:"sent"`
	Leads   []apiLeadWithScore `json:"leads"`
	Message string             `json:"message,omitempty"`
}

type leadListResponse struct {
	Leads []apiLeadWithScore `json:"leads"`
}

type updateStateRequest struct {
	State core.LeadState `json:"state"`
	Note  string         `json:"note"`
}

type updateStateResponse struct {
	ID    int64          `json:"id"`
	State core.LeadState `json:"state"`
	OK    bool           `json:"ok"`
}

func readJSON(r *http.Request, out any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(out)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

type apiLeadWithScore struct {
	Lead  apiLead      `json:"lead"`
	Score apiLeadScore `json:"score"`
}

type apiLead struct {
	ID           int64          `json:"id"`
	Source       string         `json:"source"`
	Category     core.Category  `json:"category"`
	Title        string         `json:"title"`
	Body         string         `json:"body"`
	URL          string         `json:"url"`
	Author       string         `json:"author"`
	Company      string         `json:"company"`
	Location     string         `json:"location"`
	Compensation string         `json:"compensation"`
	State        core.LeadState `json:"state"`
	CreatedAt    time.Time      `json:"created_at"`
}

type apiLeadScore struct {
	Score         int    `json:"score"`
	Rationale     string `json:"rationale"`
	DraftOpener   string `json:"draft_opener"`
	ShouldNotify  bool   `json:"should_notify"`
	Model         string `json:"model"`
	PromptVersion string `json:"prompt_version"`
}

func toAPILeadWithScores(items []core.LeadWithScore) []apiLeadWithScore {
	if len(items) == 0 {
		return []apiLeadWithScore{}
	}
	out := make([]apiLeadWithScore, 0, len(items))
	for _, item := range items {
		out = append(out, apiLeadWithScore{
			Lead:  toAPILead(item.Lead),
			Score: toAPILeadScore(item.Score),
		})
	}
	return out
}

func toAPILead(lead core.Lead) apiLead {
	return apiLead{
		ID:           lead.ID,
		Source:       lead.Source,
		Category:     lead.Category,
		Title:        lead.Title,
		Body:         lead.Body,
		URL:          lead.URL,
		Author:       lead.Author,
		Company:      lead.Company,
		Location:     lead.Location,
		Compensation: lead.Compensation,
		State:        lead.State,
		CreatedAt:    lead.CreatedAt,
	}
}

func toAPILeadScore(score core.LeadScore) apiLeadScore {
	return apiLeadScore{
		Score:         score.Score,
		Rationale:     score.Rationale,
		DraftOpener:   score.DraftOpener,
		ShouldNotify:  score.ShouldNotify,
		Model:         score.Model,
		PromptVersion: score.PromptVersion,
	}
}

func fromAPILead(lead apiLead) core.Lead {
	return core.Lead{
		ID:           lead.ID,
		Source:       lead.Source,
		Category:     lead.Category,
		Title:        lead.Title,
		Body:         lead.Body,
		URL:          lead.URL,
		Author:       lead.Author,
		Company:      lead.Company,
		Location:     lead.Location,
		Compensation: lead.Compensation,
		State:        lead.State,
		CreatedAt:    lead.CreatedAt,
	}
}

func fromAPILeadScore(score apiLeadScore) core.LeadScore {
	return core.LeadScore{
		Score:         score.Score,
		Rationale:     score.Rationale,
		DraftOpener:   score.DraftOpener,
		ShouldNotify:  score.ShouldNotify,
		Model:         score.Model,
		PromptVersion: score.PromptVersion,
	}
}
