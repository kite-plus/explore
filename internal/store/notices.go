package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Notice is a notice or ad Explore publishes itself; see
// docs/design/notices.md.
type Notice struct {
	ID         int64      `json:"id"`
	Kind       string     `json:"kind"`
	Title      string     `json:"title"`
	Summary    string     `json:"summary"`
	Body       string     `json:"body"`
	URL        string     `json:"url"`
	SourceName string     `json:"source_name"`
	Position   int        `json:"position"`
	Audience   string     `json:"audience"`
	StartsAt   *time.Time `json:"starts_at"`
	EndsAt     *time.Time `json:"ends_at"`
	Enabled    bool       `json:"enabled"`
	UpdatedBy  string     `json:"updated_by"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// NoticeInput is what a maintainer sets on a notice.
type NoticeInput struct {
	Kind       string
	Title      string
	Summary    string
	Body       string
	URL        string
	SourceName string
	Position   int
	Audience   string
	StartsAt   *time.Time
	EndsAt     *time.Time
	Enabled    bool
}

const noticeColumns = `id, kind, title, summary, body, url, source_name, position, audience,
	starts_at, ends_at, enabled, updated_by, created_at, updated_at`

// showing is the condition for a notice readers see now.
const showing = `enabled AND (starts_at IS NULL OR starts_at <= now()) AND (ends_at IS NULL OR ends_at > now())`

func scanNotice(row pgx.CollectableRow) (Notice, error) {
	var n Notice
	err := row.Scan(&n.ID, &n.Kind, &n.Title, &n.Summary, &n.Body, &n.URL, &n.SourceName, &n.Position, &n.Audience,
		&n.StartsAt, &n.EndsAt, &n.Enabled, &n.UpdatedBy, &n.CreatedAt, &n.UpdatedAt)
	return n, err
}

// Notices lists every notice for maintainers, drafts and ended ones too.
func (s *Store) Notices(ctx context.Context) ([]Notice, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+noticeColumns+` FROM notices
		ORDER BY enabled DESC, position, coalesce(starts_at, created_at) DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanNotice)
}

// ActiveNotices lists the notices showing now for an interface language,
// "zh" or "en"; an empty audience gets only the notices for everyone.
func (s *Store) ActiveNotices(ctx context.Context, audience string) ([]Notice, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+noticeColumns+` FROM notices
		WHERE `+showing+` AND (audience = '' OR audience = $1)
		ORDER BY position, coalesce(starts_at, created_at) DESC, id DESC LIMIT 20`, audience)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanNotice)
}

// ActiveNotice returns a notice that is showing now.
func (s *Store) ActiveNotice(ctx context.Context, id int64) (Notice, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+noticeColumns+` FROM notices WHERE id = $1 AND `+showing, id)
	if err != nil {
		return Notice{}, err
	}
	n, err := pgx.CollectExactlyOneRow(rows, scanNotice)
	if errors.Is(err, pgx.ErrNoRows) {
		return Notice{}, ErrNotFound
	}
	return n, err
}

func (s *Store) CreateNotice(ctx context.Context, in NoticeInput, by string) (Notice, error) {
	rows, err := s.pool.Query(ctx, `INSERT INTO notices(kind, title, summary, body, url, source_name, position, audience,
		starts_at, ends_at, enabled, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING `+noticeColumns,
		in.Kind, in.Title, in.Summary, in.Body, in.URL, in.SourceName, in.Position, in.Audience,
		in.StartsAt, in.EndsAt, in.Enabled, by)
	if err != nil {
		return Notice{}, err
	}
	return pgx.CollectExactlyOneRow(rows, scanNotice)
}

func (s *Store) UpdateNotice(ctx context.Context, id int64, in NoticeInput, by string) (Notice, error) {
	rows, err := s.pool.Query(ctx, `UPDATE notices SET kind = $2, title = $3, summary = $4, body = $5, url = $6,
		source_name = $7, position = $8, audience = $9, starts_at = $10, ends_at = $11, enabled = $12,
		updated_by = $13, updated_at = now()
		WHERE id = $1 RETURNING `+noticeColumns,
		id, in.Kind, in.Title, in.Summary, in.Body, in.URL, in.SourceName, in.Position, in.Audience,
		in.StartsAt, in.EndsAt, in.Enabled, by)
	if err != nil {
		return Notice{}, err
	}
	n, err := pgx.CollectExactlyOneRow(rows, scanNotice)
	if errors.Is(err, pgx.ErrNoRows) {
		return Notice{}, ErrNotFound
	}
	return n, err
}

func (s *Store) DeleteNotice(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM notices WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
