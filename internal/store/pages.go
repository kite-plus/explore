package store

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/kite-plus/explore/internal/policy"
)

// PageJob is an entry whose page head is due to be read.
type PageJob struct {
	EntryID int64
	BlogID  int64
	URL     string
}

// PageResult is what a page head offered. Done is false when reading failed
// in a way worth retrying.
type PageResult struct {
	ImageURL string
	Excerpt  string
	Done     bool
}

// needsPage matches entries whose feed gave no image or no full excerpt.
const needsPage = `(e.image_url IS NULL OR e.excerpt IS NULL OR ` + cutExcerpt + `)`

// ClaimPages leases up to limit due entries of visible blogs, at most one
// per blog, so a round asks each site for one page.
func (s *Store) ClaimPages(ctx context.Context, limit int) ([]PageJob, error) {
	rows, err := s.pool.Query(ctx, `
		WITH per_blog AS (
			SELECT DISTINCT ON (e.blog_id) e.id, e.page_next_check_at
			FROM entries e JOIN blogs b ON b.id = e.blog_id
			WHERE e.page_next_check_at <= now() AND `+needsPage+` AND `+visible+`
			ORDER BY e.blog_id, e.page_next_check_at, e.id
		), claimed AS (
			SELECT e.id FROM entries e JOIN per_blog p ON p.id = e.id
			ORDER BY p.page_next_check_at, p.id
			LIMIT @limit FOR UPDATE OF e SKIP LOCKED
		)
		UPDATE entries e
		SET page_next_check_at = now() + (@lease * interval '1 second')
		FROM claimed WHERE e.id = claimed.id
		RETURNING e.id, e.blog_id, e.url`, pgx.NamedArgs{
		"limit": limit, "lease": seconds(policy.PageCheckLease), "unhealthy_after": seconds(policy.UnhealthyAfter),
	})
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (PageJob, error) {
		var job PageJob
		err := row.Scan(&job.EntryID, &job.BlogID, &job.URL)
		return job, err
	})
}

// RecordPage stores what a page head offered. A finished read is not repeated
// until the entry's URL changes; a failed one is retried after
// policy.PageRetryInterval. A job whose entry moved to another URL since it
// was claimed is dropped.
func (s *Store) RecordPage(ctx context.Context, job PageJob, r PageResult) error {
	if !r.Done {
		_, err := s.pool.Exec(ctx, `
			UPDATE entries SET page_next_check_at = now() + (@retry * interval '1 second')
			WHERE id = @id AND url = @url`,
			pgx.NamedArgs{"id": job.EntryID, "url": job.URL, "retry": seconds(policy.PageRetryInterval)})
		return err
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE entries
		SET page_image_url = nullif(@image, ''), page_excerpt = nullif(@excerpt, ''), page_next_check_at = NULL
		WHERE id = @id AND url = @url`,
		pgx.NamedArgs{"id": job.EntryID, "url": job.URL, "image": r.ImageURL, "excerpt": r.Excerpt})
	return err
}
