package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/kite-plus/explore/internal/model"
	"github.com/kite-plus/explore/internal/policy"
)

// NewBlog is a blog about to be listed.
type NewBlog struct {
	Host         string
	Reviewer     string
	Name         string
	SiteURL      string
	FeedURL      string
	Language     string
	Generator    model.Generator
	ShowExcerpt  bool
	ExtraDomains []string
}

// CreateBlog lists a blog directly, as a maintainer.
func (s *Store) CreateBlog(ctx context.Context, nb NewBlog) (model.Blog, error) {
	var blog model.Blog
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		if err = lockHost(ctx, tx, nb.Host); err != nil {
			return err
		}
		blog, err = createBlog(ctx, tx, nb)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			UPDATE submissions
			SET status = 'approved', blog_id = $2, reviewed_by = coalesce(nullif($3, ''), 'system'), reviewed_at = now()
			WHERE host = $1 AND status = 'pending'`, nb.Host, blog.ID, nb.Reviewer)
		return err
	})
	return blog, err
}

// createBlog refuses hosts that opted out or were blocked, even when the
// exclusion came after the submission.
func createBlog(ctx context.Context, tx pgx.Tx, nb NewBlog) (model.Blog, error) {
	var excluded bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM excluded_hosts WHERE host = $1)`, nb.Host).Scan(&excluded); err != nil {
		return model.Blog{}, err
	}
	if excluded {
		return model.Blog{}, ErrExcluded
	}
	if nb.ExtraDomains == nil {
		nb.ExtraDomains = []string{}
	}
	if nb.Generator == "" {
		nb.Generator = model.GeneratorUnknown
	}
	blog, err := scanBlog(tx.QueryRow(ctx, `
		INSERT INTO blogs (host, name, site_url, feed_url, language, generator, show_excerpt, extra_domains)
		VALUES (@host, @name, @site_url, @feed_url, @language, @generator, @show_excerpt, @extra_domains)
		ON CONFLICT (host) DO NOTHING
		RETURNING `+blogColumns,
		pgx.NamedArgs{
			"host": nb.Host, "name": nb.Name, "site_url": nb.SiteURL, "feed_url": nb.FeedURL,
			"language": nb.Language, "generator": string(nb.Generator),
			"show_excerpt": nb.ShowExcerpt, "extra_domains": nb.ExtraDomains,
		}))
	if errors.Is(err, ErrNotFound) {
		return model.Blog{}, ErrListed
	}
	return blog, err
}

// Blogs lists every blog with its fetch state, for maintainers. With
// unhealthy set, only blogs readers cannot currently see are listed.
func (s *Store) Blogs(ctx context.Context, unhealthy bool, limit int) ([]model.Blog, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+blogColumns+` FROM blogs b
		WHERE NOT @unhealthy OR NOT (`+visible+`)
		ORDER BY host LIMIT @limit`,
		pgx.NamedArgs{"unhealthy": unhealthy, "unhealthy_after": seconds(policy.UnhealthyAfter), "limit": limit})
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.Blog, error) { return scanBlog(row) })
}

// Blog returns one blog with its fetch state.
func (s *Store) Blog(ctx context.Context, host string) (model.Blog, error) {
	return scanBlog(s.pool.QueryRow(ctx, `SELECT `+blogColumns+` FROM blogs WHERE host = $1`, host))
}

// BlogUpdate changes a blog; nil fields stay as they are.
type BlogUpdate struct {
	Name         *string
	Language     *string
	FeedURL      *string
	ExtraDomains *[]string
	ShowExcerpt  *bool
	Status       *model.BlogStatus
	StatusNote   *string
	DefaultTags  *[]string
}

// UpdateBlog applies a maintainer's changes. Changing anything that shapes
// normalization forgets the fetch cache, so the next fetch rebuilds the
// snapshot instead of finding the feed unchanged. New default tags send the
// blog's entries back to the tagger with the new hint.
func (s *Store) UpdateBlog(ctx context.Context, host string, u BlogUpdate) (model.Blog, error) {
	var status *string
	if u.Status != nil {
		st := string(*u.Status)
		status = &st
	}
	reset := u.FeedURL != nil || u.ExtraDomains != nil || u.ShowExcerpt != nil
	var blog model.Blog
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		blog, err = updateBlog(ctx, tx, host, u, status, reset)
		if err != nil || u.DefaultTags == nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE entries SET tagged_at = NULL WHERE blog_id = $1`, blog.ID)
		return err
	})
	return blog, err
}

func updateBlog(ctx context.Context, tx pgx.Tx, host string, u BlogUpdate, status *string, reset bool) (model.Blog, error) {
	return scanBlog(tx.QueryRow(ctx, `
		UPDATE blogs SET
			name          = coalesce(@name, name),
			language      = coalesce(@language, language),
			feed_url      = coalesce(@feed_url, feed_url),
			extra_domains = coalesce(@extra_domains, extra_domains),
			show_excerpt  = coalesce(@show_excerpt, show_excerpt),
			default_tags  = coalesce(@default_tags, default_tags),
			status        = coalesce(@status, status),
			status_note   = CASE WHEN @set_note THEN nullif(@status_note, '') ELSE status_note END,
			etag          = CASE WHEN @reset THEN NULL ELSE etag END,
			last_modified = CASE WHEN @reset THEN NULL ELSE last_modified END,
			body_hash     = CASE WHEN @reset THEN NULL ELSE body_hash END,
			next_fetch_at = CASE WHEN @reset THEN now() ELSE next_fetch_at END,
			updated_at    = now()
		WHERE host = @host
		RETURNING `+blogColumns,
		pgx.NamedArgs{
			"host": host, "name": u.Name, "language": u.Language, "feed_url": u.FeedURL,
			"extra_domains": u.ExtraDomains, "show_excerpt": u.ShowExcerpt, "status": status, "default_tags": u.DefaultTags,
			"set_note": u.StatusNote != nil, "status_note": deref(u.StatusNote), "reset": reset,
		}))
}

// DeleteBlog removes a blog and, through the foreign key, its entries.
// With a reason, the host is also excluded from being listed again.
func (s *Store) DeleteBlog(ctx context.Context, host string, exclude model.ExclusionReason, note string) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM blogs WHERE host = $1`, host)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		if exclude == "" {
			return nil
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO excluded_hosts (host, reason, note) VALUES ($1, $2, nullif($3, ''))
			ON CONFLICT (host) DO UPDATE SET reason = excluded.reason, note = excluded.note`,
			host, string(exclude), note)
		return err
	})
}

// FetchNow makes a blog due immediately.
func (s *Store) FetchNow(ctx context.Context, host string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE blogs SET next_fetch_at = now() WHERE host = $1`, host)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// SetDescription records a metadata check. An empty result leaves the last
// useful description in place while still delaying the next check.
func (s *Store) SetDescription(ctx context.Context, blogID int64, description string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE blogs SET description = coalesce(nullif($2, ''), description),
		    description_checked_at = now(), updated_at = now()
		WHERE id = $1`, blogID, description)
	return err
}

// ExcludedHosts lists every exclusion, newest first.
func (s *Store) ExcludedHosts(ctx context.Context) ([]model.ExcludedHost, error) {
	rows, err := s.pool.Query(ctx, `SELECT host, reason, coalesce(note, ''), created_at FROM excluded_hosts ORDER BY created_at DESC, host`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.ExcludedHost, error) {
		var e model.ExcludedHost
		err := row.Scan(&e.Host, &e.Reason, &e.Note, &e.CreatedAt)
		return e, err
	})
}

// DeleteExcludedHost lets a host be listed again.
func (s *Store) DeleteExcludedHost(ctx context.Context, host string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM excluded_hosts WHERE host = $1`, host)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
