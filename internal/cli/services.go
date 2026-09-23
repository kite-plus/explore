package cli

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/kite-plus/explore/internal/api"
	"github.com/kite-plus/explore/internal/buildinfo"
	"github.com/kite-plus/explore/internal/check"
	"github.com/kite-plus/explore/internal/config"
	"github.com/kite-plus/explore/internal/fetch"
	"github.com/kite-plus/explore/internal/store"
	"github.com/kite-plus/explore/internal/tagger"
	"github.com/kite-plus/explore/internal/worker"
)

// openStore loads the configuration, requires a database and connects.
func openStore(ctx context.Context) (config.Config, *store.Store, *slog.Logger, error) {
	cfg, err := loadConfig()
	if err != nil {
		return cfg, nil, nil, err
	}
	if err := cfg.RequireDatabase(); err != nil {
		return cfg, nil, nil, err
	}
	log := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return cfg, nil, nil, err
	}
	return cfg, st, log, nil
}

func newFetcher(cfg config.Config) *fetch.Client {
	return fetch.New(fetch.Options{
		UserAgent:    fetch.UserAgent(buildinfo.Version, cfg.PublicURL),
		AllowPrivate: cfg.AllowPrivateNetworks,
	})
}

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the HTTP API",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg, st, log, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer st.Close()

			admins := make([]api.AdminToken, 0, len(cfg.AdminTokens))
			for _, t := range cfg.AdminTokens {
				admins = append(admins, api.AdminToken{Name: t.Name, Hash: t.Hash})
			}
			srv := &api.Server{
				Store:          st,
				Checker:        &check.Checker{Fetch: newFetcher(cfg)},
				ImageFetch:     fetch.New(fetch.Options{UserAgent: fetch.UserAgent(buildinfo.Version, cfg.PublicURL), AllowPrivate: cfg.AllowPrivateNetworks, Timeout: 8 * time.Second}),
				LinkFetch:      newFetcher(cfg),
				PublicURL:      cfg.PublicURL,
				Admins:         admins,
				TrustedProxies: cfg.TrustedProxies,
				AllowPrivate:   cfg.AllowPrivateNetworks,
				Log:            log,
			}
			h, err := srv.Handler()
			if err != nil {
				return err
			}
			// Submissions run a check of up to 30 seconds, hence the write
			// timeout well above it.
			hs := &http.Server{
				Addr:              cfg.HTTPAddr,
				Handler:           h,
				ReadHeaderTimeout: 10 * time.Second,
				ReadTimeout:       30 * time.Second,
				WriteTimeout:      60 * time.Second,
				IdleTimeout:       120 * time.Second,
			}
			errc := make(chan error, 1)
			go func() { errc <- hs.ListenAndServe() }()
			log.Info("serving", "addr", cfg.HTTPAddr, "admin_api", len(admins) > 0, "version", buildinfo.Version)

			select {
			case err := <-errc:
				if errors.Is(err, http.ErrServerClosed) {
					return nil
				}
				return err
			case <-ctx.Done():
				shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				log.Info("shutting down")
				return hs.Shutdown(shutdown)
			}
		},
	}
}

func newWorkerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "worker",
		Short: "Fetch due feeds and run daily maintenance",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg, st, log, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer st.Close()
			w := &worker.Worker{Store: st, Fetch: newFetcher(cfg), Log: log, Concurrency: cfg.WorkerConcurrency}
			if cfg.Tagger.Enabled() {
				w.Tagger = entryTagger{tagger.New(tagger.Options{
					APIKey: cfg.Tagger.APIKey, Model: cfg.Tagger.Model, Effort: cfg.Tagger.Effort,
				})}
			}
			log.Info("worker started", "concurrency", cfg.WorkerConcurrency, "tagger_model", cfg.Tagger.Model,
				"version", buildinfo.Version)
			return w.Run(ctx)
		},
	}
}

func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Manage the database schema",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "up",
		Short: "Apply every pending migration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, st, _, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer st.Close()
			if err := st.Migrate(cmd.Context()); err != nil {
				return err
			}
			printf(cmd, "migrations applied\n")
			return nil
		},
	})
	return cmd
}

// entryTagger hands the worker's tag jobs to the tagger.
type entryTagger struct{ t *tagger.Tagger }

func (e entryTagger) Tag(ctx context.Context, j store.TagJob) ([]string, error) {
	return e.t.Tag(ctx, tagger.Post{
		Title: j.Title, Excerpt: j.Excerpt, Categories: j.Categories, Language: j.Language, BlogTags: j.BlogTags,
	})
}
