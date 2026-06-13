package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	"lead-scout/internal/api"
	"lead-scout/internal/collectors"
	"lead-scout/internal/config"
	"lead-scout/internal/core"
	"lead-scout/internal/db"
	"lead-scout/internal/normalize"
	"lead-scout/internal/scoring"
	"lead-scout/internal/telegram"
)

func Run(ctx context.Context, cfg config.Config, args []string) error {
	if len(args) == 0 {
		return usage()
	}

	switch args[0] {
	case "migrate":
		return withRepo(ctx, cfg, func(conn *db.Repository) error {
			return nil
		}, true)
	case "collect":
		return collect(ctx, cfg, args[1:])
	case "score":
		return score(ctx, cfg, args[1:])
	case "digest":
		return digest(ctx, cfg, args[1:])
	case "bot":
		return bot(ctx, cfg)
	case "serve":
		return serve(ctx, cfg, args[1:])
	default:
		return usage()
	}
}

func usage() error {
	return errors.New(`usage:
  lead-scout migrate
  lead-scout collect --source hn
  lead-scout collect --all
  lead-scout score --since 24h
  lead-scout digest --daily
  lead-scout bot
  lead-scout serve --addr :8080`)
}

func collect(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("collect", flag.ContinueOnError)
	source := fs.String("source", "", "source name")
	all := fs.Bool("all", false, "collect all sources")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*all && *source == "" {
		return errors.New("collect requires --source or --all")
	}

	return withRepo(ctx, cfg, func(repo *db.Repository) error {
		registry := collectors.Registry(cfg)
		var selected []core.Collector
		if *all {
			for _, name := range []string{"hn", "braintrust", "remoteok", "wwr", "reddit"} {
				selected = append(selected, registry[name])
			}
		} else {
			c, ok := registry[*source]
			if !ok {
				return fmt.Errorf("unknown source %q", *source)
			}
			selected = append(selected, c)
		}

		normalizer := normalize.New()
		for _, collector := range selected {
			if collector == nil {
				continue
			}
			rawItems, err := collector.Fetch(ctx)
			if err != nil {
				if errors.Is(err, collectors.ErrNotConfigured) && *all {
					fmt.Printf("skip %s: not configured\n", collector.Name())
					continue
				}
				return fmt.Errorf("collect %s: %w", collector.Name(), err)
			}
			fmt.Printf("%s: fetched %d raw items\n", collector.Name(), len(rawItems))
			for _, raw := range rawItems {
				savedRaw, err := repo.UpsertRawItem(ctx, raw)
				if err != nil {
					return err
				}
				leads, err := normalizer.Normalize(savedRaw)
				if err != nil {
					return err
				}
				for _, lead := range leads {
					if lead.Title == "" {
						continue
					}
					if _, err := repo.UpsertLead(ctx, lead); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}, false)
}

func score(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("score", flag.ContinueOnError)
	sinceRaw := fs.String("since", "24h", "duration to look back")
	batchDelay := fs.Duration("batch-delay", 2*time.Second, "delay between AI API calls")
	if err := fs.Parse(args); err != nil {
		return err
	}
	since, err := time.ParseDuration(*sinceRaw)
	if err != nil {
		return err
	}

	return withRepo(ctx, cfg, func(repo *db.Repository) error {
		leads, err := repo.PendingScoreLeads(ctx, since)
		if err != nil {
			return err
		}
		heuristic := scoring.NewHeuristic()
		aiScorer := scoring.NewNVIDIA(cfg.NVIDIAAPIKey, cfg.NVIDIABaseURL, cfg.NVIDIAModel, heuristic)
		notifier := telegram.New(cfg.TelegramBotToken, cfg.TelegramChatID)

		aiCalls := 0
		for i, lead := range leads {
			hScore, err := heuristic.Score(ctx, lead)
			if err != nil {
				return err
			}
			finalScore := hScore
			if scoring.ShouldDeepScore(hScore) {
				// Add delay between AI calls to avoid rate limiting
				if aiCalls > 0 {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(*batchDelay):
					}
				}
				finalScore, err = aiScorer.Score(ctx, lead)
				if err != nil {
					return err
				}
				aiCalls++
				fmt.Printf("[%d/%d] AI scored lead %d: %d\n", i+1, len(leads), lead.ID, finalScore.Score)
			}
			finalScore.LeadID = lead.ID
			saved, err := repo.InsertLeadScore(ctx, finalScore)
			if err != nil {
				return err
			}
			if notifier.Configured() && lead.Category == core.CategoryGig && saved.ShouldNotify && saved.Score >= 80 {
				if err := notifier.SendHotLead(ctx, lead, saved); err != nil {
					return err
				}
				if err := repo.RecordEvent(ctx, lead.ID, "notified", "hot gig alert sent", map[string]string{"channel": "telegram"}); err != nil {
					return err
				}
			}
		}
		fmt.Printf("scored %d leads (%d AI calls)\n", len(leads), aiCalls)
		return nil
	}, false)
}

func digest(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("digest", flag.ContinueOnError)
	daily := fs.Bool("daily", false, "send daily digest")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*daily {
		return errors.New("digest requires --daily")
	}
	if !cfg.TelegramConfigured() {
		return errors.New("telegram is not configured")
	}

	return withRepo(ctx, cfg, func(repo *db.Repository) error {
		categories := []core.Category{core.CategoryFounder, core.CategoryGig}
		candidates, err := repo.DigestCandidates(ctx, categories, 50, 20)
		if err != nil {
			return err
		}
		notifier := telegram.New(cfg.TelegramBotToken, cfg.TelegramChatID)
		if err := notifier.SendDigest(ctx, candidates); err != nil {
			return err
		}
		for _, item := range candidates {
			if err := repo.RecordEvent(ctx, item.Lead.ID, "digest_sent", "daily founder digest", map[string]string{"channel": "telegram"}); err != nil {
				return err
			}
		}
		fmt.Printf("sent digest with %d leads\n", len(candidates))
		return nil
	}, false)
}

func bot(ctx context.Context, cfg config.Config) error {
	if !cfg.TelegramConfigured() {
		return errors.New("telegram is not configured")
	}
	return withRepo(ctx, cfg, func(repo *db.Repository) error {
		client := telegram.New(cfg.TelegramBotToken, cfg.TelegramChatID)
		var offset int64
		for {
			updates, err := client.GetUpdates(ctx, offset)
			if err != nil {
				return err
			}
			for _, update := range updates {
				if update.UpdateID >= offset {
					offset = update.UpdateID + 1
				}
				if update.CallbackQuery == nil {
					continue
				}
				action, err := telegram.ParseAction(update.CallbackQuery.Data)
				if err != nil {
					_ = client.AnswerCallback(ctx, update.CallbackQuery.ID, "Unknown action")
					continue
				}
				if err := repo.SetLeadState(ctx, action.LeadID, action.State, "telegram action"); err != nil {
					_ = client.AnswerCallback(ctx, update.CallbackQuery.ID, "Could not update lead")
					continue
				}
				_ = client.AnswerCallback(ctx, update.CallbackQuery.ID, "Updated")
			}
		}
	}, false)
}

func serve(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", cfg.APIAddr, "HTTP listen address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := cfg.ValidateDB(); err != nil {
		return err
	}
	conn, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		return err
	}
	fmt.Printf("serving API docs at http://localhost%s/docs\n", displayAddr(*addr))
	return api.ListenAndServe(ctx, cfg, db.NewRepository(conn), *addr)
}

func withRepo(ctx context.Context, cfg config.Config, fn func(*db.Repository) error, migrateOnly bool) error {
	if err := cfg.ValidateDB(); err != nil {
		return err
	}
	conn, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer conn.Close()

	if migrateOnly {
		if err := db.Migrate(ctx, conn); err != nil {
			return err
		}
		fmt.Println("migrations applied")
		return nil
	}
	return fn(db.NewRepository(conn))
}

func displayAddr(addr string) string {
	if addr == "" || addr == ":8080" {
		return ":8080"
	}
	if addr[0] == ':' {
		return addr
	}
	return ""
}
