package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/kite-plus/explore/internal/policy"
)

// SitemapJob is a blog whose sitemap is due to be read.
type SitemapJob struct {
	BlogID  int64
	Host    string
	Name    string
	SiteURL string
	FeedURL string
}

// SitemapURL is a post address a sitemap lists.
type SitemapURL struct {
	URL     string
	LastMod *time.Time
}

// ClaimSitemaps leases up to limit visible blogs whose sitemap is due.
func (s *Store) ClaimSitemaps(ctx context.Context, limit int) ([]SitemapJob, error) {
	rows, err := s.pool.Query(ctx, `
		WITH due AS (
			SELECT b.id FROM blogs b
			WHERE b.sitemap_next_check_at <= now() AND `+visible+`
			ORDER BY b.sitemap_next_check_at, b.id
			LIMIT @limit FOR UPDATE OF b SKIP LOCKED
		)
		UPDATE blogs b SET sitemap_next_check_at = now() + (@lease * interval '1 second')
		FROM due WHERE b.id = due.id
		RETURNING b.id, b.host, b.name, b.site_url, b.feed_url`, pgx.NamedArgs{
		"limit": limit, "lease": seconds(policy.SitemapLease), "unhealthy_after": seconds(policy.UnhealthyAfter),
	})
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (SitemapJob, error) {
		var j SitemapJob
		err := row.Scan(&j.BlogID, &j.Host, &j.Name, &j.SiteURL, &j.FeedURL)
		return j, err
	})
}

// FeedEntryURLs returns the links of a blog's feed entries, the examples its
// post addresses are learned from.
func (s *Store) FeedEntryURLs(ctx context.Context, blogID int64) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT url FROM entries WHERE blog_id = $1 AND source = 'feed'`, blogID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// SyncSitemap makes urls a blog's sitemap addresses: new ones, and ones with
// a new lastmod, become due to be read; ones no longer listed go, with the
// entries read from them. The sitemap is due again at next.
func (s *Store) SyncSitemap(ctx context.Context, blogID int64, urls []SitemapURL, next time.Time) error {
	addresses := make([]string, 0, len(urls))
	lastmods := make([]*time.Time, 0, len(urls))
	seen := make(map[string]bool, len(urls))
	for _, u := range urls {
		if seen[u.URL] {
			continue
		}
		seen[u.URL] = true
		addresses = append(addresses, u.URL)
		lastmods = append(lastmods, u.LastMod)
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		args := pgx.NamedArgs{"blog_id": blogID, "urls": addresses, "lastmods": lastmods, "next": next}
		if _, err := tx.Exec(ctx, `
			INSERT INTO sitemap_urls (blog_id, url, lastmod)
			SELECT @blog_id, u.url, u.lastmod FROM unnest(@urls::text[], @lastmods::timestamptz[]) AS u (url, lastmod)
			ON CONFLICT (blog_id, url) DO UPDATE
			SET lastmod = excluded.lastmod,
			    next_check_at = CASE WHEN excluded.lastmod IS DISTINCT FROM sitemap_urls.lastmod THEN now()
			                         ELSE sitemap_urls.next_check_at END`, args); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM entries WHERE blog_id = @blog_id AND source = 'sitemap' AND NOT (url = ANY (@urls::text[]))`, args); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM sitemap_urls WHERE blog_id = @blog_id AND NOT (url = ANY (@urls::text[]))`, args); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE blogs SET sitemap_next_check_at = @next WHERE id = @blog_id`, args)
		return err
	})
}

// PostponeSitemap sets when a blog's sitemap is next tried, after it could
// not be found or read.
func (s *Store) PostponeSitemap(ctx context.Context, blogID int64, next time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE blogs SET sitemap_next_check_at = $2 WHERE id = $1`, blogID, next)
	return err
}

// SitemapPageJob is a sitemap address whose page is due to be read.
type SitemapPageJob struct {
	BlogID   int64
	BlogName string
	URL      string
}

// ClaimSitemapPages leases up to limit due sitemap addresses, newest first,
// at most one per blog and none of the blogs in skip, which this round
// already asks. An address a feed entry covers waits: the feed says more
// about the post, and the address is read once the feed drops it.
func (s *Store) ClaimSitemapPages(ctx context.Context, limit int, skip []int64) ([]SitemapPageJob, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
		WITH per_blog AS (
			SELECT DISTINCT ON (su.blog_id) su.blog_id, su.url, su.next_check_at
			FROM sitemap_urls su JOIN blogs b ON b.id = su.blog_id
			WHERE su.next_check_at <= now() AND `+visible+`
			  AND NOT (su.blog_id = ANY (@skip::bigint[]))
			  AND NOT EXISTS (SELECT 1 FROM entries f WHERE f.blog_id = su.blog_id AND f.source = 'feed' AND f.url_key = su.url_key)
			ORDER BY su.blog_id, su.lastmod DESC NULLS LAST, su.url
		), claimed AS (
			SELECT su.blog_id, su.url FROM sitemap_urls su JOIN per_blog p ON p.blog_id = su.blog_id AND p.url = su.url
			ORDER BY p.next_check_at, p.blog_id
			LIMIT @limit FOR UPDATE OF su SKIP LOCKED
		)
		UPDATE sitemap_urls su SET next_check_at = now() + (@lease * interval '1 second')
		FROM claimed c, blogs b
		WHERE su.blog_id = c.blog_id AND su.url = c.url AND b.id = su.blog_id
		RETURNING su.blog_id, b.name, su.url`, pgx.NamedArgs{
		"limit": limit, "skip": skip, "lease": seconds(policy.PageCheckLease), "unhealthy_after": seconds(policy.UnhealthyAfter),
	})
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (SitemapPageJob, error) {
		var j SitemapPageJob
		err := row.Scan(&j.BlogID, &j.BlogName, &j.URL)
		return j, err
	})
}

// SitemapPage is what a sitemap address's page said. Post is false for a
// page that is no post or asks not to be listed; Done is false when reading
// failed in a way worth retrying.
type SitemapPage struct {
	Done        bool
	Post        bool
	Identity    string
	Title       string
	PublishedAt *time.Time
	DateTrusted bool
	ImageURL    string
	Excerpt     string
}

// RecordSitemapPage stores a post read from a sitemap address as an entry,
// unless a feed entry covers it by now, and marks the address read.
func (s *Store) RecordSitemapPage(ctx context.Context, job SitemapPageJob, p SitemapPage) error {
	args := pgx.NamedArgs{
		"blog_id": job.BlogID, "url": job.URL, "identity": p.Identity, "title": p.Title,
		"published": p.PublishedAt, "trusted": p.DateTrusted, "image": p.ImageURL, "excerpt": p.Excerpt,
		"retry": seconds(policy.PageRetryInterval), "link_every": seconds(policy.SitemapLinkCheckInterval),
	}
	if !p.Done {
		_, err := s.pool.Exec(ctx, `
			UPDATE sitemap_urls SET next_check_at = now() + (@retry * interval '1 second')
			WHERE blog_id = @blog_id AND url = @url`, args)
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if p.Post {
			if _, err := tx.Exec(ctx, `
				INSERT INTO entries (blog_id, identity, url, title, published_at, date_trusted, source,
				                     page_image_url, page_excerpt, page_next_check_at,
				                     link_status, link_checked_at, link_next_check_at)
				SELECT @blog_id, @identity, @url, @title, @published, @trusted, 'sitemap',
				       nullif(@image, ''), nullif(@excerpt, ''), NULL,
				       'available', now(), now() + (@link_every * interval '1 second')
				WHERE NOT EXISTS (SELECT 1 FROM entries f
				                  WHERE f.blog_id = @blog_id AND f.source = 'feed' AND f.url_key = entry_url_key(@url))
				ON CONFLICT (blog_id, identity) DO UPDATE
				SET url            = excluded.url,
				    title          = excluded.title,
				    published_at   = excluded.published_at,
				    date_trusted   = excluded.date_trusted,
				    page_image_url = excluded.page_image_url,
				    page_excerpt   = excluded.page_excerpt,
				    tags           = CASE WHEN entries.title = excluded.title THEN entries.tags ELSE '{}' END,
				    tagged_at      = CASE WHEN entries.title = excluded.title THEN entries.tagged_at END,
				    synced_at      = now()
				WHERE entries.source = 'sitemap'`, args); err != nil {
				return err
			}
		} else if _, err := tx.Exec(ctx, `
			DELETE FROM entries WHERE blog_id = @blog_id AND identity = @identity AND source = 'sitemap'`, args); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE sitemap_urls SET next_check_at = NULL WHERE blog_id = @blog_id AND url = @url`, args)
		return err
	})
}
