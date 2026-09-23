package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/kite-plus/explore/internal/model"
	"github.com/kite-plus/explore/internal/policy"
)

// BlogRef is the part of a blog shown next to its entries.
type BlogRef struct {
	Host     string
	Name     string
	SiteURL  string
	FeedURL  string
	Language string
}

// StreamEntry is an entry of the home stream.
type StreamEntry struct {
	model.Entry
	Blog BlogRef
}

// Cursor is a position in a list ordered by time, then id.
type Cursor struct {
	At time.Time
	ID int64
}

// StreamQuery selects a page of the home stream.
type StreamQuery struct {
	Lang   string // a lower-case language prefix such as "zh", or empty
	Tag    string // a tag slug, or empty
	Limit  int
	Cursor *Cursor
}

// Stream returns the home stream: trusted, recent, not in the future, and
// at most policy.StreamPerBlogPerDay entries per blog and day. The daily
// cap is applied before the cursor so pages stay consistent.
func (s *Store) Stream(ctx context.Context, q StreamQuery) ([]StreamEntry, error) {
	args := pgx.NamedArgs{
		"unhealthy_after":  seconds(policy.UnhealthyAfter),
		"window":           seconds(policy.StreamWindow),
		"future_tolerance": seconds(policy.FutureTolerance),
		"per_day":          policy.StreamPerBlogPerDay,
		"lang":             q.Lang,
		"tag":              q.Tag,
		"has_cursor":       q.Cursor != nil,
		"cursor_at":        time.Time{},
		"cursor_id":        int64(0),
		"limit":            q.Limit,
	}
	if q.Cursor != nil {
		args["cursor_at"], args["cursor_id"] = q.Cursor.At, q.Cursor.ID
	}
	rows, err := s.pool.Query(ctx, `
		WITH ranked AS (
			SELECT e.id, e.blog_id, e.identity, e.url, e.title, coalesce(e.excerpt, '') AS excerpt, coalesce(e.image_url, '') AS image_url, e.published_at, e.tags,
			       b.host, b.name, b.site_url, b.feed_url, b.language,
			       row_number() OVER (
			           PARTITION BY e.blog_id, date_trunc('day', e.published_at AT TIME ZONE 'UTC')
			           ORDER BY e.published_at DESC, e.id DESC
			       ) AS rank_in_day
			FROM entries e
			JOIN blogs b ON b.id = e.blog_id
			WHERE `+visible+`
			  AND e.date_trusted
			  AND e.published_at >  now() - (@window * interval '1 second')
			  AND e.published_at <= now() + (@future_tolerance * interval '1 second')
			  AND (@lang::text = '' OR lower(b.language) = @lang OR lower(b.language) LIKE @lang || '-%')
			  AND (@tag::text = '' OR @tag = ANY (e.tags))
		)
		SELECT id, blog_id, identity, url, title, excerpt, image_url, published_at, tags, host, name, site_url, feed_url, language
		FROM ranked
		WHERE rank_in_day <= @per_day
		  AND (NOT @has_cursor OR (published_at, id) < (@cursor_at, @cursor_id))
		ORDER BY published_at DESC, id DESC
		LIMIT @limit`, args)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (StreamEntry, error) {
		var e StreamEntry
		err := row.Scan(&e.ID, &e.BlogID, &e.Identity, &e.URL, &e.Title, &e.Excerpt, &e.ImageURL, &e.PublishedAt, &e.Tags,
			&e.Blog.Host, &e.Blog.Name, &e.Blog.SiteURL, &e.Blog.FeedURL, &e.Blog.Language)
		e.DateTrusted = true
		return e, err
	})
}

// ListedBlog is a visible blog as readers see it.
type ListedBlog struct {
	ID              int64
	Host            string
	Name            string
	Description     string
	SiteURL         string
	FeedURL         string
	Language        string
	Generator       model.Generator
	LastPublishedAt *time.Time
}

// DirectoryQuery selects a page of the blog directory.
type DirectoryQuery struct {
	Lang   string
	Limit  int
	Cursor *Cursor // At is the epoch for blogs with no dated entry
}

// Directory lists visible blogs, most recently published first. Blogs with
// nothing dated sort last, keyed on the epoch so the cursor stays simple.
func (s *Store) Directory(ctx context.Context, q DirectoryQuery) ([]ListedBlog, error) {
	args := pgx.NamedArgs{
		"unhealthy_after":  seconds(policy.UnhealthyAfter),
		"future_tolerance": seconds(policy.FutureTolerance),
		"lang":             q.Lang,
		"has_cursor":       q.Cursor != nil,
		"cursor_at":        time.Time{},
		"cursor_id":        int64(0),
		"limit":            q.Limit,
	}
	if q.Cursor != nil {
		args["cursor_at"], args["cursor_id"] = q.Cursor.At, q.Cursor.ID
	}
	rows, err := s.pool.Query(ctx, `
		WITH listed AS (
			SELECT b.id, b.host, b.name, b.description, b.site_url, b.feed_url, b.language, b.generator,
			       (SELECT max(e.published_at) FROM entries e
			        WHERE e.blog_id = b.id AND e.date_trusted
			          AND e.published_at <= now() + (@future_tolerance * interval '1 second')) AS last_published_at
			FROM blogs b
			WHERE `+visible+`
			  AND (@lang::text = '' OR lower(b.language) = @lang OR lower(b.language) LIKE @lang || '-%')
		)
		SELECT id, host, name, description, site_url, feed_url, language, generator, last_published_at
		FROM listed
		WHERE NOT @has_cursor OR (coalesce(last_published_at, 'epoch'), id) < (@cursor_at, @cursor_id)
		ORDER BY coalesce(last_published_at, 'epoch') DESC, id DESC
		LIMIT @limit`, args)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanListed)
}

func scanListed(row pgx.CollectableRow) (ListedBlog, error) {
	var b ListedBlog
	err := row.Scan(&b.ID, &b.Host, &b.Name, &b.Description, &b.SiteURL, &b.FeedURL, &b.Language, &b.Generator, &b.LastPublishedAt)
	return b, err
}

// VisibleBlog returns a visible blog and every cached entry that is not in
// the future, undated ones last.
func (s *Store) VisibleBlog(ctx context.Context, host string) (ListedBlog, []model.Entry, error) {
	args := pgx.NamedArgs{
		"host":             host,
		"unhealthy_after":  seconds(policy.UnhealthyAfter),
		"future_tolerance": seconds(policy.FutureTolerance),
	}
	rows, err := s.pool.Query(ctx, `
		SELECT b.id, b.host, b.name, b.description, b.site_url, b.feed_url, b.language, b.generator,
		       (SELECT max(e.published_at) FROM entries e
		        WHERE e.blog_id = b.id AND e.date_trusted
		          AND e.published_at <= now() + (@future_tolerance * interval '1 second'))
		FROM blogs b
		WHERE b.host = @host AND `+visible, args)
	if err != nil {
		return ListedBlog{}, nil, err
	}
	blog, err := pgx.CollectExactlyOneRow(rows, scanListed)
	if errors.Is(err, pgx.ErrNoRows) {
		return ListedBlog{}, nil, ErrNotFound
	}
	if err != nil {
		return ListedBlog{}, nil, err
	}

	rows, err = s.pool.Query(ctx, `
		SELECT id, blog_id, identity, url, title, coalesce(excerpt, ''), coalesce(image_url, ''), published_at, date_trusted, tags
		FROM entries
		WHERE blog_id = @id AND (published_at IS NULL OR published_at <= now() + (@future_tolerance * interval '1 second'))
		ORDER BY published_at DESC NULLS LAST, id`,
		pgx.NamedArgs{"id": blog.ID, "future_tolerance": seconds(policy.FutureTolerance)})
	if err != nil {
		return ListedBlog{}, nil, err
	}
	entries, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.Entry, error) {
		var e model.Entry
		err := row.Scan(&e.ID, &e.BlogID, &e.Identity, &e.URL, &e.Title, &e.Excerpt, &e.ImageURL, &e.PublishedAt, &e.DateTrusted, &e.Tags)
		return e, err
	})
	return blog, entries, err
}
