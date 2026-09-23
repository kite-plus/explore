package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/kite-plus/explore/internal/model"
)

// Claimed is a blog the worker took for one fetch.
type Claimed struct {
	ID                   int64
	Host                 string
	SiteURL              string
	FeedURL              string
	DescriptionCheckedAt *time.Time
	ETag                 string
	LastModified         string
	BodyHash             []byte
	FetchInterval        time.Duration
	ConsecutiveFailures  int
	ShowExcerpt          bool
	ExtraDomains         []string
	// HasEntries is false after the cache was emptied, which tells the
	// worker to skip conditional requests and rebuild the snapshot.
	HasEntries bool
}

// ClaimDue takes up to batch due blogs and pushes their next fetch a lease
// into the future, so a worker that dies mid-fetch only delays them.
func (s *Store) ClaimDue(ctx context.Context, batch int, lease time.Duration) ([]Claimed, error) {
	rows, err := s.pool.Query(ctx, `
		UPDATE blogs
		SET next_fetch_at = now() + (@lease * interval '1 second')
		WHERE id IN (
			SELECT id FROM blogs
			WHERE status = 'active' AND next_fetch_at <= now()
			ORDER BY next_fetch_at
			LIMIT @batch
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, host, site_url, feed_url, description_checked_at, coalesce(etag, ''), coalesce(last_modified, ''), body_hash,
		          extract(epoch FROM fetch_interval)::bigint, consecutive_failures,
		          show_excerpt, extra_domains,
		          EXISTS (SELECT 1 FROM entries e WHERE e.blog_id = blogs.id)`,
		pgx.NamedArgs{"lease": seconds(lease), "batch": batch})
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Claimed, error) {
		var c Claimed
		var interval int64
		err := row.Scan(&c.ID, &c.Host, &c.SiteURL, &c.FeedURL, &c.DescriptionCheckedAt, &c.ETag, &c.LastModified, &c.BodyHash,
			&interval, &c.ConsecutiveFailures, &c.ShowExcerpt, &c.ExtraDomains, &c.HasEntries)
		c.FetchInterval = time.Duration(interval) * time.Second
		return c, err
	})
}

// FetchState is what a successful fetch leaves behind.
type FetchState struct {
	ETag          string
	LastModified  string
	BodyHash      []byte
	FetchInterval time.Duration
	NextFetchAt   time.Time
	// FeedURL replaces the stored feed address after a permanent redirect;
	// empty keeps it.
	FeedURL   string
	Generator model.Generator
}

// SyncSnapshot makes a blog's entries equal its current feed: new items
// are added, changed ones updated and vanished ones deleted, in one
// transaction. See docs/design/data-model.md section 3.
func (s *Store) SyncSnapshot(ctx context.Context, blogID int64, entries []model.Entry, st FetchState) error {
	n := len(entries)
	identities, urls, titles, excerpts, images := make([]string, n), make([]string, n), make([]string, n), make([]string, n), make([]string, n)
	published, trusted := make([]*time.Time, n), make([]bool, n)
	// unnest cannot take one array per row, so each row's categories travel
	// as a JSON array.
	categories := make([]string, n)
	for i, e := range entries {
		identities[i], urls[i], titles[i], excerpts[i] = e.Identity, e.URL, e.Title, e.Excerpt
		images[i] = e.ImageURL
		published[i], trusted[i] = e.PublishedAt, e.DateTrusted
		c, err := json.Marshal(append([]string{}, e.Categories...))
		if err != nil {
			return err
		}
		categories[i] = string(c)
	}

	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var id int64
		if err := tx.QueryRow(ctx, `SELECT id FROM blogs WHERE id = $1 FOR UPDATE`, blogID).Scan(&id); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO entries (blog_id, identity, url, title, excerpt, image_url, published_at, date_trusted, categories)
			SELECT @blog_id, s.identity, s.url, s.title, nullif(s.excerpt, ''), nullif(s.image_url, ''), s.published_at, s.date_trusted,
			       ARRAY(SELECT jsonb_array_elements_text(s.categories::jsonb))
			FROM unnest(@identities::text[], @urls::text[], @titles::text[],
			            @excerpts::text[], @images::text[], @published::timestamptz[], @trusted::boolean[], @categories::text[])
			     AS s (identity, url, title, excerpt, image_url, published_at, date_trusted, categories)
			ON CONFLICT (blog_id, identity) DO UPDATE
			SET url          = excluded.url,
			    title        = excluded.title,
			    excerpt      = excluded.excerpt,
			    image_url    = excluded.image_url,
			    published_at = excluded.published_at,
			    date_trusted = excluded.date_trusted,
			    categories   = excluded.categories,
			    link_status = CASE WHEN entries.url = excluded.url THEN entries.link_status ELSE 'unknown' END,
			    link_checked_at = CASE WHEN entries.url = excluded.url THEN entries.link_checked_at END,
			    link_next_check_at = CASE WHEN entries.url = excluded.url THEN entries.link_next_check_at ELSE now() END,
			    -- Tags belong to the title they were given for; a new title
			    -- is tagged again.
			    tags         = CASE WHEN entries.title = excluded.title THEN entries.tags ELSE '{}' END,
			    tagged_at    = CASE WHEN entries.title = excluded.title THEN entries.tagged_at END,
			    synced_at    = now()`,
			pgx.NamedArgs{
				"blog_id": blogID, "identities": identities, "urls": urls, "titles": titles,
				"excerpts": excerpts, "images": images, "published": published, "trusted": trusted, "categories": categories,
			}); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM entries
			WHERE blog_id = @blog_id AND NOT (identity = ANY (@identities::text[]))`,
			pgx.NamedArgs{"blog_id": blogID, "identities": identities}); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			UPDATE blogs
			SET etag = nullif(@etag, ''), last_modified = nullif(@last_modified, ''), body_hash = @body_hash,
			    fetch_interval = @interval * interval '1 second', next_fetch_at = @next_fetch_at,
			    feed_url = coalesce(nullif(@feed_url, ''), feed_url),
			    generator = coalesce(nullif(@generator, ''), generator),
			    last_fetched_at = now(), last_succeeded_at = now(),
			    consecutive_failures = 0, last_error = NULL, gone_since = NULL,
			    updated_at = now()
			WHERE id = @blog_id`,
			pgx.NamedArgs{
				"blog_id": blogID, "etag": st.ETag, "last_modified": st.LastModified, "body_hash": st.BodyHash,
				"interval": seconds(st.FetchInterval), "next_fetch_at": st.NextFetchAt,
				"feed_url": st.FeedURL, "generator": string(st.Generator),
			})
		return err
	})
}

// RecordUnchanged notes a fetch that found the feed as it was. The entries
// stay; only the fetch state moves on.
func (s *Store) RecordUnchanged(ctx context.Context, blogID int64, st FetchState) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE blogs
		SET etag = coalesce(nullif(@etag, ''), etag),
		    last_modified = coalesce(nullif(@last_modified, ''), last_modified),
		    fetch_interval = @interval * interval '1 second', next_fetch_at = @next_fetch_at,
		    last_fetched_at = now(), last_succeeded_at = now(),
		    consecutive_failures = 0, last_error = NULL, gone_since = NULL,
		    updated_at = now()
		WHERE id = @blog_id`,
		pgx.NamedArgs{
			"blog_id": blogID, "etag": st.ETag, "last_modified": st.LastModified,
			"interval": seconds(st.FetchInterval), "next_fetch_at": st.NextFetchAt,
		})
	return err
}

// Failure is a fetch that did not produce a snapshot.
type Failure struct {
	Error       string
	NextFetchAt time.Time
	// Gone marks an exit signal: 410 Gone, or robots.txt refusing us.
	Gone bool
}

// RecordFailure keeps the entries and backs off. Health is derived from
// last_succeeded_at, so nothing else changes here.
func (s *Store) RecordFailure(ctx context.Context, blogID int64, f Failure) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE blogs
		SET consecutive_failures = consecutive_failures + 1, last_error = @error,
		    next_fetch_at = @next_fetch_at, last_fetched_at = now(),
		    gone_since = CASE WHEN @gone THEN coalesce(gone_since, now()) ELSE gone_since END,
		    updated_at = now()
		WHERE id = @blog_id`,
		pgx.NamedArgs{"blog_id": blogID, "error": f.Error, "next_fetch_at": f.NextFetchAt, "gone": f.Gone})
	return err
}

// ClearCache empties the entries table. Nothing is lost: blogs without
// entries are fetched in full, so each comes back on its next fetch. See
// docs/design/architecture.md section 0.1.
func (s *Store) ClearCache(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `TRUNCATE entries`)
	return err
}

// Maintenance reports what a maintenance pass did.
type Maintenance struct {
	Ran     bool
	Removed []string
	Purged  int64
}

// Maintain removes blogs that have said they are gone since before
// goneBefore, recording them as opted out, and purges reviewed submissions
// older than purgeBefore. An advisory lock keeps it to one worker at a time.
func (s *Store) Maintain(ctx context.Context, goneBefore, purgeBefore time.Time) (Maintenance, error) {
	var m Maintenance
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtext('explore.maintenance'))`).Scan(&m.Ran); err != nil || !m.Ran {
			return err
		}
		rows, err := tx.Query(ctx, `DELETE FROM blogs WHERE gone_since < $1 RETURNING host`, goneBefore)
		if err != nil {
			return err
		}
		if m.Removed, err = pgx.CollectRows(rows, pgx.RowTo[string]); err != nil {
			return err
		}
		if len(m.Removed) > 0 {
			if _, err := tx.Exec(ctx, `
				INSERT INTO excluded_hosts (host, reason, note)
				SELECT unnest($1::text[]), 'opt_out', 'the site signaled it is gone'
				ON CONFLICT (host) DO NOTHING`, m.Removed); err != nil {
				return err
			}
		}
		tag, err := tx.Exec(ctx, `
			DELETE FROM submissions
			WHERE status <> 'pending' AND coalesce(reviewed_at, created_at) < $1`, purgeBefore)
		m.Purged = tag.RowsAffected()
		return err
	})
	return m, err
}
