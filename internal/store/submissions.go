package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/kite-plus/explore/internal/model"
)

// HostState is what the store knows about a host before a submission.
type HostState struct {
	Listed    bool
	Excluded  *model.ExcludedHost
	PendingID string
}

// HostState looks a host up in the blog list, the exclusions and the
// pending submissions.
func (s *Store) HostState(ctx context.Context, host string) (HostState, error) {
	var st HostState
	var reason, note *string
	var excludedAt *time.Time
	var pending *string
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM blogs WHERE host = $1),
		       (SELECT reason FROM excluded_hosts WHERE host = $1),
		       (SELECT coalesce(note, '') FROM excluded_hosts WHERE host = $1),
		       (SELECT created_at FROM excluded_hosts WHERE host = $1),
		       (SELECT id::text FROM submissions WHERE host = $1 AND status = 'pending')`,
		host).Scan(&st.Listed, &reason, &note, &excludedAt, &pending)
	if err != nil {
		return st, err
	}
	if reason != nil {
		st.Excluded = &model.ExcludedHost{Host: host, Reason: model.ExclusionReason(*reason), Note: *note, CreatedAt: *excludedAt}
	}
	if pending != nil {
		st.PendingID = *pending
	}
	return st, nil
}

// CreateSubmission stores a submission that passed its check. The report
// is stored without hints; they are added per language when shown.
func (s *Store) CreateSubmission(ctx context.Context, sub model.Submission) (model.Submission, error) {
	report := sub.Report
	report.Problems = append([]model.Problem(nil), report.Problems...)
	for i := range report.Problems {
		report.Problems[i].Hint = ""
	}
	raw, err := json.Marshal(report)
	if err != nil {
		return model.Submission{}, err
	}
	var created model.Submission
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := lockHost(ctx, tx, sub.Host); err != nil {
			return err
		}
		var listed bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM blogs WHERE host = $1)`, sub.Host).Scan(&listed); err != nil {
			return err
		}
		if listed {
			return ErrListed
		}
		var err error
		created, err = scanSubmission(tx.QueryRow(ctx, `
			INSERT INTO submissions (host, site_url, feed_url, note, check_report)
			VALUES (@host, @site_url, @feed_url, nullif(@note, ''), @report)
			RETURNING `+submissionColumns,
			pgx.NamedArgs{"host": sub.Host, "site_url": sub.SiteURL, "feed_url": sub.FeedURL, "note": sub.Note, "report": raw}))
		if isUniqueViolation(err) {
			return ErrPending
		}
		return err
	})
	return created, err
}

// Submission returns one submission. Anything that is not a UUID is simply
// not found.
func (s *Store) Submission(ctx context.Context, id string) (model.Submission, error) {
	if !isUUID(id) {
		return model.Submission{}, ErrNotFound
	}
	return scanSubmission(s.pool.QueryRow(ctx, `SELECT `+submissionColumns+` FROM submissions WHERE id = $1::uuid`, id))
}

// Submissions lists submissions with a status, oldest first so the queue
// is worked in order.
func (s *Store) Submissions(ctx context.Context, status model.SubmissionStatus, limit int) ([]model.Submission, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+submissionColumns+` FROM submissions
		WHERE status = $1 ORDER BY created_at, id LIMIT $2`, string(status), limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.Submission, error) { return scanSubmission(row) })
}

// Approval holds a reviewer's choices when approving; empty fields fall
// back to what the check found.
type Approval struct {
	Reviewer     string
	Name         string
	Language     string
	FeedURL      string
	ExtraDomains []string
	ShowExcerpt  *bool
}

// ApproveSubmission lists the submitted blog and closes the submission in
// one transaction.
func (s *Store) ApproveSubmission(ctx context.Context, id string, a Approval) (model.Blog, error) {
	if !isUUID(id) {
		return model.Blog{}, ErrNotFound
	}
	var blog model.Blog
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var host string
		if err := tx.QueryRow(ctx, `SELECT host FROM submissions WHERE id = $1::uuid`, id).Scan(&host); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if err := lockHost(ctx, tx, host); err != nil {
			return err
		}
		sub, err := scanSubmission(tx.QueryRow(ctx, `SELECT `+submissionColumns+` FROM submissions WHERE id = $1::uuid FOR UPDATE`, id))
		if err != nil {
			return err
		}
		if sub.Status != model.SubmissionPending {
			return ErrNotPending
		}
		nb := NewBlog{
			Host:         sub.Host,
			Name:         firstOf(a.Name, sub.Report.Title, sub.Host),
			SiteURL:      sub.SiteURL,
			FeedURL:      firstOf(a.FeedURL, sub.FeedURL),
			Language:     firstOf(a.Language, sub.Report.Language, "und"),
			Generator:    sub.Report.Generator,
			ShowExcerpt:  a.ShowExcerpt == nil || *a.ShowExcerpt,
			ExtraDomains: a.ExtraDomains,
		}
		if blog, err = createBlog(ctx, tx, nb); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			UPDATE submissions
			SET status = 'approved', reviewed_by = $2, reviewed_at = now(), blog_id = $3
			WHERE id = $1::uuid`, id, a.Reviewer, blog.ID)
		return err
	})
	return blog, err
}

func lockHost(ctx context.Context, tx pgx.Tx, host string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(1570, hashtext($1))`, host)
	return err
}

// RejectSubmission closes a pending submission with a note the author sees.
func (s *Store) RejectSubmission(ctx context.Context, id, reviewer, note string) error {
	if !isUUID(id) {
		return ErrNotFound
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var status string
		err := tx.QueryRow(ctx, `SELECT status FROM submissions WHERE id = $1::uuid FOR UPDATE`, id).Scan(&status)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if status != string(model.SubmissionPending) {
			return ErrNotPending
		}
		_, err = tx.Exec(ctx, `
			UPDATE submissions
			SET status = 'rejected', review_note = $2, reviewed_by = $3, reviewed_at = now()
			WHERE id = $1::uuid`, id, note, reviewer)
		return err
	})
}

const submissionColumns = `id::text, host, site_url, feed_url, coalesce(note, ''), check_report, status,
	coalesce(review_note, ''), coalesce(reviewed_by, ''), reviewed_at, blog_id, created_at`

func scanSubmission(row pgx.Row) (model.Submission, error) {
	var sub model.Submission
	var raw []byte
	err := row.Scan(&sub.ID, &sub.Host, &sub.SiteURL, &sub.FeedURL, &sub.Note, &raw, &sub.Status,
		&sub.ReviewNote, &sub.ReviewedBy, &sub.ReviewedAt, &sub.BlogID, &sub.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Submission{}, ErrNotFound
	}
	if err != nil {
		return model.Submission{}, err
	}
	err = json.Unmarshal(raw, &sub.Report)
	return sub, err
}

func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch {
		case i == 8 || i == 13 || i == 18 || i == 23:
			if c != '-' {
				return false
			}
		case (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F'):
		default:
			return false
		}
	}
	return true
}

func firstOf(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
