package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type FetchAttempt struct {
	ID         int64
	StartedAt  time.Time
	FinishedAt *time.Time
	Outcome    string
	HTTPStatus *int
	EntryCount *int
	Error      string
}

type FetchQueueItem struct {
	Host                string
	Name                string
	Status              string
	NextFetchAt         time.Time
	LastFetchedAt       *time.Time
	LastSucceededAt     *time.Time
	ConsecutiveFailures int
	LastError           string
	LastAttempt         *FetchAttempt
}

func (s *Store) FetchQueue(ctx context.Context, limit int) ([]FetchQueueItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT b.host, b.name, b.status, b.next_fetch_at, b.last_fetched_at, b.last_succeeded_at,
		       b.consecutive_failures, coalesce(b.last_error, ''),
		       a.id, a.started_at, a.finished_at, a.outcome, a.http_status, a.entry_count, coalesce(a.error, '')
		FROM blogs b
		LEFT JOIN LATERAL (
			SELECT id, started_at, finished_at, outcome, http_status, entry_count, error
			FROM fetch_attempts WHERE blog_id = b.id ORDER BY id DESC LIMIT 1
		) a ON true
		ORDER BY CASE WHEN b.status = 'paused' THEN 1 ELSE 0 END, b.next_fetch_at, b.host
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (FetchQueueItem, error) {
		var item FetchQueueItem
		var id *int64
		var startedAt *time.Time
		var finishedAt *time.Time
		var outcome *string
		var httpStatus *int
		var entryCount *int
		var attemptError string
		err := row.Scan(&item.Host, &item.Name, &item.Status, &item.NextFetchAt,
			&item.LastFetchedAt, &item.LastSucceededAt, &item.ConsecutiveFailures, &item.LastError,
			&id, &startedAt, &finishedAt, &outcome, &httpStatus, &entryCount, &attemptError)
		if err == nil && id != nil && startedAt != nil && outcome != nil {
			item.LastAttempt = &FetchAttempt{
				ID: *id, StartedAt: *startedAt, FinishedAt: finishedAt,
				Outcome: *outcome, HTTPStatus: httpStatus, EntryCount: entryCount, Error: attemptError,
			}
		}
		return item, err
	})
}

func (s *Store) FetchAttempts(ctx context.Context, host string, limit int) ([]FetchAttempt, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, a.started_at, a.finished_at, a.outcome, a.http_status, a.entry_count, coalesce(a.error, '')
		FROM fetch_attempts a JOIN blogs b ON b.id = a.blog_id
		WHERE b.host = $1 ORDER BY a.id DESC LIMIT $2`, host, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (FetchAttempt, error) {
		var attempt FetchAttempt
		err := row.Scan(&attempt.ID, &attempt.StartedAt, &attempt.FinishedAt, &attempt.Outcome,
			&attempt.HTTPStatus, &attempt.EntryCount, &attempt.Error)
		return attempt, err
	})
}

func (s *Store) FinishFetchAttempt(ctx context.Context, id int64, outcome string, httpStatus, entryCount *int, reason string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE fetch_attempts
		SET finished_at = now(), outcome = $2, http_status = $3, entry_count = $4, error = nullif($5, '')
		WHERE id = $1 AND outcome = 'running'`, id, outcome, httpStatus, entryCount, reason)
	return err
}

func (s *Store) RecordWorkerHeartbeat(ctx context.Context, workerID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO worker_heartbeats (worker_id) VALUES ($1)
		ON CONFLICT (worker_id) DO UPDATE SET last_seen_at = now()`, workerID)
	return err
}

func (s *Store) RemoveWorkerHeartbeat(ctx context.Context, workerID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM worker_heartbeats WHERE worker_id = $1`, workerID)
	return err
}

func (s *Store) WorkerHeartbeats(ctx context.Context) (int, *time.Time, error) {
	var online int
	var lastSeen *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE last_seen_at > now() - interval '90 seconds'), max(last_seen_at)
		FROM worker_heartbeats`).Scan(&online, &lastSeen)
	return online, lastSeen, err
}
