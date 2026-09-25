package store

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// TagJob is an entry waiting for tags, with what the tagger reads.
type TagJob struct {
	EntryID    int64
	Title      string
	Excerpt    string
	Categories []string
	Language   string
	BlogTags   []string // the maintainer's default tags for the blog
}

// Untagged returns entries of active blogs that have not been tagged yet,
// newest first, so a rebuilt cache fills the stream from the top.
func (s *Store) Untagged(ctx context.Context, limit int) ([]TagJob, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.title, coalesce(e.excerpt, e.page_excerpt, ''), e.categories, b.language, b.default_tags
		FROM entries e
		JOIN blogs b ON b.id = e.blog_id
		WHERE e.tagged_at IS NULL AND b.status = 'active' AND b.gone_since IS NULL
		ORDER BY e.published_at DESC NULLS LAST, e.id DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (TagJob, error) {
		var j TagJob
		err := row.Scan(&j.EntryID, &j.Title, &j.Excerpt, &j.Categories, &j.Language, &j.BlogTags)
		return j, err
	})
}

// SetTags records an entry's tags. It does nothing when the entry is gone
// or its title changed since the job was read: the new title waits for its
// own turn.
func (s *Store) SetTags(ctx context.Context, entryID int64, title string, tags []string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE entries SET tags = $3, tagged_at = now()
		WHERE id = $1 AND title = $2`, entryID, title, append([]string{}, tags...))
	return err
}
