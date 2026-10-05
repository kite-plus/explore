package store

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/kite-plus/explore/internal/model"
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
// newest first, so a rebuilt cache fills the stream from the top. Only the
// newest window entries of those blogs are considered, tagged or not, so
// the window moves on as new posts come in.
func (s *Store) Untagged(ctx context.Context, limit, window int) ([]TagJob, error) {
	rows, err := s.pool.Query(ctx, `
		WITH recent AS (
			SELECT e.id, e.published_at, e.tagged_at
			FROM entries e
			JOIN blogs b ON b.id = e.blog_id
			WHERE b.status = 'active' AND b.gone_since IS NULL
			ORDER BY e.published_at DESC NULLS LAST, e.id DESC
			LIMIT $2
		)
		SELECT e.id, e.title, coalesce(e.excerpt, e.page_excerpt, ''), e.categories, b.language, b.default_tags
		FROM recent r
		JOIN entries e ON e.id = r.id
		JOIN blogs b ON b.id = e.blog_id
		WHERE r.tagged_at IS NULL
		ORDER BY r.published_at DESC NULLS LAST, r.id DESC
		LIMIT $1`, limit, window)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (TagJob, error) {
		var j TagJob
		err := row.Scan(&j.EntryID, &j.Title, &j.Excerpt, &j.Categories, &j.Language, &j.BlogTags)
		return j, err
	})
}

// SetRating records an entry's tags and quality. It does nothing when the
// entry is gone or its title changed since the job was read: the new title
// waits for its own turn.
func (s *Store) SetRating(ctx context.Context, entryID int64, title string, r model.Rating) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE entries SET tags = $3, quality = $4, tagged_at = now()
		WHERE id = $1 AND title = $2`, entryID, title, append([]string{}, r.Tags...), int16(r.Quality))
	return err
}
