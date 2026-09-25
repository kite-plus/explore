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
	Blog   BlogRef
	SortAt time.Time
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

// Stream returns the latest stream: every entry with a trusted date that is
// not in the future, newest first, and at most policy.StreamPerBlogPerDay
// entries per blog and day. The daily cap is applied before the cursor so
// pages stay consistent.
func (s *Store) Stream(ctx context.Context, q StreamQuery) ([]StreamEntry, error) {
	args := pgx.NamedArgs{
		"unhealthy_after":  seconds(policy.UnhealthyAfter),
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
			SELECT e.id, e.blog_id, e.identity, e.url, e.title, coalesce(`+shownExcerpt+`, '') AS excerpt, coalesce(`+shownImage+`, '') AS image_url, e.published_at, e.tags, e.link_status, e.link_checked_at,
			       b.host, b.name, b.site_url, b.feed_url, b.language,
			       row_number() OVER (
			           PARTITION BY e.blog_id, date_trunc('day', e.published_at AT TIME ZONE 'UTC')
			           ORDER BY e.published_at DESC, e.id DESC
			       ) AS rank_in_day
			FROM entries e
			JOIN blogs b ON b.id = e.blog_id
			WHERE `+visible+`
			  AND NOT EXISTS (SELECT 1 FROM suppressed_entries se WHERE se.blog_id = e.blog_id AND se.identity = e.identity)
			  AND e.date_trusted
			  AND e.published_at <= now() + (@future_tolerance * interval '1 second')
			  AND (@lang::text = '' OR lower(b.language) = @lang OR lower(b.language) LIKE @lang || '-%')
			  AND (@tag::text = '' OR @tag = ANY (e.tags))
		)
		SELECT id, blog_id, identity, url, title, excerpt, image_url, published_at, tags, link_status, link_checked_at, host, name, site_url, feed_url, language
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
		err := row.Scan(&e.ID, &e.BlogID, &e.Identity, &e.URL, &e.Title, &e.Excerpt, &e.ImageURL, &e.PublishedAt, &e.Tags, &e.LinkStatus, &e.LinkCheckedAt,
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
			          AND NOT EXISTS (SELECT 1 FROM suppressed_entries se WHERE se.blog_id = e.blog_id AND se.identity = e.identity)
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

// BlogPageQuery selects a page of a blog's entries; a Limit of 0 takes them
// all.
type BlogPageQuery struct {
	Limit  int
	Cursor *Cursor
}

// EntrySortAt is where an entry sorts on its blog's page and in the page
// cursor: its publish date, or the epoch for an undated one, which puts it
// last.
func EntrySortAt(e model.Entry) time.Time {
	if e.PublishedAt == nil {
		return time.Unix(0, 0).UTC()
	}
	return *e.PublishedAt
}

// VisibleBlog returns a visible blog and every cached entry that is not in
// the future.
func (s *Store) VisibleBlog(ctx context.Context, host string) (ListedBlog, []model.Entry, error) {
	return s.VisibleBlogPage(ctx, host, BlogPageQuery{})
}

// VisibleBlogPage returns a visible blog and a page of its entries that are
// not in the future, newest first and undated ones last (EntrySortAt).
func (s *Store) VisibleBlogPage(ctx context.Context, host string, q BlogPageQuery) (ListedBlog, []model.Entry, error) {
	args := pgx.NamedArgs{
		"host":             host,
		"unhealthy_after":  seconds(policy.UnhealthyAfter),
		"future_tolerance": seconds(policy.FutureTolerance),
	}
	rows, err := s.pool.Query(ctx, `
		SELECT b.id, b.host, b.name, b.description, b.site_url, b.feed_url, b.language, b.generator,
		       (SELECT max(e.published_at) FROM entries e
		        WHERE e.blog_id = b.id AND e.date_trusted
		          AND NOT EXISTS (SELECT 1 FROM suppressed_entries se WHERE se.blog_id = e.blog_id AND se.identity = e.identity)
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
		SELECT e.id, e.blog_id, e.identity, e.url, e.title, coalesce(`+shownExcerpt+`, ''), coalesce(`+shownImage+`, ''),
		       e.published_at, e.date_trusted, e.tags, e.link_status, e.link_checked_at
		FROM entries e
		WHERE e.blog_id = @id AND NOT EXISTS (SELECT 1 FROM suppressed_entries se WHERE se.blog_id = e.blog_id AND se.identity = e.identity)
		  AND (e.published_at IS NULL OR e.published_at <= now() + (@future_tolerance * interval '1 second'))
		  AND (NOT @has_cursor OR (coalesce(e.published_at, 'epoch'), e.id) < (@cursor_at, @cursor_id))
		ORDER BY coalesce(e.published_at, 'epoch') DESC, e.id DESC
		LIMIT @limit`,
		pgx.NamedArgs{"id": blog.ID, "future_tolerance": seconds(policy.FutureTolerance), "limit": limitOrAll(q.Limit),
			"has_cursor": q.Cursor != nil, "cursor_at": cursorAt(q.Cursor), "cursor_id": cursorID(q.Cursor)})
	if err != nil {
		return ListedBlog{}, nil, err
	}
	entries, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.Entry, error) {
		var e model.Entry
		err := row.Scan(&e.ID, &e.BlogID, &e.Identity, &e.URL, &e.Title, &e.Excerpt, &e.ImageURL, &e.PublishedAt, &e.DateTrusted, &e.Tags, &e.LinkStatus, &e.LinkCheckedAt)
		return e, err
	})
	return blog, entries, err
}

// limitOrAll turns a Limit of 0 into SQL's LIMIT NULL, which takes every row.
func limitOrAll(limit int) any {
	if limit <= 0 {
		return nil
	}
	return limit
}

func cursorAt(c *Cursor) time.Time {
	if c == nil {
		return time.Time{}
	}
	return c.At
}

func cursorID(c *Cursor) int64 {
	if c == nil {
		return 0
	}
	return c.ID
}
