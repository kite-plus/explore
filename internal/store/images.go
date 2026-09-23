package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/kite-plus/explore/internal/policy"
)

// VisibleImageURL returns the source URL only while its entry belongs to a
// visible blog. The browser never receives this URL.
func (s *Store) VisibleImageURL(ctx context.Context, entryID int64) (string, error) {
	var source string
	err := s.pool.QueryRow(ctx, `
		SELECT e.image_url
		FROM entries e JOIN blogs b ON b.id = e.blog_id
		WHERE e.id = @entry_id AND e.image_url IS NOT NULL
		  AND `+visible+`
		  AND (e.published_at IS NULL OR e.published_at <= now() + (@future_tolerance * interval '1 second'))`,
		pgx.NamedArgs{"entry_id": entryID, "unhealthy_after": seconds(policy.UnhealthyAfter),
			"future_tolerance": seconds(policy.FutureTolerance)}).Scan(&source)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return source, err
}

// VisibleBlogSiteURL returns the site URL only while the blog is public.
func (s *Store) VisibleBlogSiteURL(ctx context.Context, host string) (string, error) {
	var siteURL string
	err := s.pool.QueryRow(ctx, `
		SELECT b.site_url FROM blogs b
		WHERE b.host = @host AND `+visible,
		pgx.NamedArgs{"host": host, "unhealthy_after": seconds(policy.UnhealthyAfter)}).Scan(&siteURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return siteURL, err
}
