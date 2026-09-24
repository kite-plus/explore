package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/kite-plus/explore/internal/model"
	"github.com/kite-plus/explore/internal/policy"
)

type LinkJob struct {
	EntryID int64
	URL     string
}

type LinkState struct {
	Status    model.LinkStatus
	CheckedAt *time.Time
	Checking  bool
}

// ClaimLinkOnDemand reserves an unchecked visible entry before a reader's
// request probes it. The same lease used by the worker prevents duplicate
// requests while the probe is running.
func (s *Store) ClaimLinkOnDemand(ctx context.Context, entryID int64) (LinkJob, bool, error) {
	var job LinkJob
	err := s.pool.QueryRow(ctx, `
		UPDATE entries e
		SET link_next_check_at = now() + (@lease * interval '1 second')
		FROM blogs b
		WHERE e.id = @id AND e.blog_id = b.id AND `+visible+`
		  AND NOT EXISTS (SELECT 1 FROM suppressed_entries se WHERE se.blog_id = e.blog_id AND se.identity = e.identity)
		  AND (e.published_at IS NULL OR e.published_at <= now() + (@future_tolerance * interval '1 second'))
		  AND e.link_checked_at IS NULL AND e.link_next_check_at <= now()
		RETURNING e.id, e.url`, pgx.NamedArgs{
		"id": entryID, "lease": seconds(policy.LinkCheckLease),
		"unhealthy_after":  seconds(policy.UnhealthyAfter),
		"future_tolerance": seconds(policy.FutureTolerance),
	}).Scan(&job.EntryID, &job.URL)
	if errors.Is(err, pgx.ErrNoRows) {
		return LinkJob{}, false, nil
	}
	return job, err == nil, err
}

func (s *Store) VisibleLinkState(ctx context.Context, entryID int64) (LinkState, error) {
	var state LinkState
	err := s.pool.QueryRow(ctx, `
		SELECT e.link_status, e.link_checked_at,
		       e.link_checked_at IS NULL AND e.link_next_check_at > now()
		FROM entries e JOIN blogs b ON b.id = e.blog_id
		WHERE e.id = @id AND `+visible+`
		  AND NOT EXISTS (SELECT 1 FROM suppressed_entries se WHERE se.blog_id = e.blog_id AND se.identity = e.identity)
		  AND (e.published_at IS NULL OR e.published_at <= now() + (@future_tolerance * interval '1 second'))`,
		pgx.NamedArgs{"id": entryID, "unhealthy_after": seconds(policy.UnhealthyAfter),
			"future_tolerance": seconds(policy.FutureTolerance)}).Scan(&state.Status, &state.CheckedAt, &state.Checking)
	if errors.Is(err, pgx.ErrNoRows) {
		return LinkState{}, ErrNotFound
	}
	return state, err
}

func (s *Store) ClaimLinks(ctx context.Context, limit int) ([]LinkJob, error) {
	rows, err := s.pool.Query(ctx, `
		WITH per_blog AS (
			SELECT DISTINCT ON (e.blog_id) e.id, e.blog_id, e.link_next_check_at
			FROM entries e JOIN blogs b ON b.id = e.blog_id
			WHERE e.link_next_check_at <= now() AND `+visible+`
			ORDER BY e.blog_id, e.link_next_check_at, e.id
		), claimed AS (
			SELECT e.id FROM entries e JOIN per_blog p ON p.id = e.id
			ORDER BY p.link_next_check_at, p.id
			LIMIT @limit FOR UPDATE OF e SKIP LOCKED
		)
		UPDATE entries e
		SET link_next_check_at = now() + (@lease * interval '1 second')
		FROM claimed WHERE e.id = claimed.id
		RETURNING e.id, e.url`, pgx.NamedArgs{
		"limit": limit, "lease": seconds(policy.LinkCheckLease), "unhealthy_after": seconds(policy.UnhealthyAfter),
	})
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (LinkJob, error) {
		var job LinkJob
		err := row.Scan(&job.EntryID, &job.URL)
		return job, err
	})
}

func (s *Store) RecordLinkStatus(ctx context.Context, job LinkJob, status model.LinkStatus, interval time.Duration) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE entries
		SET link_status = @status, link_checked_at = now(),
		    link_next_check_at = now() + (@interval * interval '1 second')
		WHERE id = @id AND url = @url`, pgx.NamedArgs{
		"id": job.EntryID, "url": job.URL, "status": status, "interval": seconds(interval),
	})
	return err
}
