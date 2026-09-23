// Package store is Explore's only way into PostgreSQL. Every query lives
// here, written by hand; docs/design/data-model.md describes the schema.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/kite-plus/explore/internal/model"
	"github.com/kite-plus/explore/migrations"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrListed     = errors.New("blog is already listed")
	ErrPending    = errors.New("a submission for this host is already pending")
	ErrNotPending = errors.New("submission has already been reviewed")
	ErrExcluded   = errors.New("host is excluded")
)

// Store is a PostgreSQL connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// Open connects to the database at url.
func Open(ctx context.Context, url string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("database url: %w", err)
	}
	return openConfig(ctx, cfg)
}

// OpenSchema connects with every session confined to one schema, which is
// how tests keep out of each other's way.
func OpenSchema(ctx context.Context, url, schema string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("database url: %w", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	return openConfig(ctx, cfg)
}

func openConfig(ctx context.Context, cfg *pgxpool.Config) (*Store, error) {
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

// Close releases every connection.
func (s *Store) Close() { s.pool.Close() }

// Ping checks that the database answers.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Migrate applies every pending migration.
func (s *Store) Migrate(ctx context.Context) error {
	db := stdlib.OpenDBFromPool(s.pool)
	defer func() { _ = db.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return err
	}
	_, err = p.Up(ctx)
	return err
}

func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

func seconds(d time.Duration) int64 { return int64(d / time.Second) }

// blogColumns and scanBlog read a whole blogs row.
const blogColumns = `id, host, name, description, site_url, feed_url, language, generator, status,
	coalesce(status_note, ''), show_excerpt, extra_domains, default_tags,
	coalesce(etag, ''), coalesce(last_modified, ''), body_hash,
	extract(epoch FROM fetch_interval)::bigint, next_fetch_at, last_fetched_at,
	last_succeeded_at, consecutive_failures, coalesce(last_error, ''), gone_since,
	created_at, updated_at`

func scanBlog(row pgx.Row) (model.Blog, error) {
	var b model.Blog
	var interval int64
	err := row.Scan(&b.ID, &b.Host, &b.Name, &b.Description, &b.SiteURL, &b.FeedURL, &b.Language, &b.Generator, &b.Status,
		&b.StatusNote, &b.ShowExcerpt, &b.ExtraDomains, &b.DefaultTags,
		&b.ETag, &b.LastModified, &b.BodyHash,
		&interval, &b.NextFetchAt, &b.LastFetchedAt,
		&b.LastSucceededAt, &b.ConsecutiveFailures, &b.LastError, &b.GoneSince,
		&b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Blog{}, ErrNotFound
	}
	b.FetchInterval = time.Duration(interval) * time.Second
	if b.ExtraDomains == nil {
		b.ExtraDomains = []string{}
	}
	if b.DefaultTags == nil {
		b.DefaultTags = []string{}
	}
	return b, err
}

// visible is the condition for a blog to appear anywhere readers look:
// active, not gone, and fetched successfully within policy.UnhealthyAfter.
const visible = `b.status = 'active' AND b.gone_since IS NULL
	AND b.last_succeeded_at > now() - (@unhealthy_after * interval '1 second')`
