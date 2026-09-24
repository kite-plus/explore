package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type AdminStats struct {
	Blogs              int64 `json:"blogs"`
	Entries            int64 `json:"entries"`
	Users              int64 `json:"users"`
	PendingSubmissions int64 `json:"pending_submissions"`
	PendingTakedowns   int64 `json:"pending_takedowns"`
	FailingBlogs       int64 `json:"failing_blogs"`
	DueFetches         int64 `json:"due_fetches"`
}

func (s *Store) AdminOverview(ctx context.Context) (AdminStats, error) {
	var stats AdminStats
	err := s.pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM blogs),
		(SELECT count(*) FROM entries),
		(SELECT count(*) FROM users),
		(SELECT count(*) FROM submissions WHERE status = 'pending'),
		(SELECT count(*) FROM takedown_requests WHERE status = 'pending'),
		(SELECT count(*) FROM blogs WHERE consecutive_failures > 0),
		(SELECT count(*) FROM blogs WHERE status = 'active' AND next_fetch_at <= now())`).Scan(
		&stats.Blogs, &stats.Entries, &stats.Users, &stats.PendingSubmissions,
		&stats.PendingTakedowns, &stats.FailingBlogs, &stats.DueFetches)
	return stats, err
}

type AdminUser struct {
	ID                string     `json:"id"`
	Email             string     `json:"email"`
	DisplayName       string     `json:"display_name"`
	IsAdmin           bool       `json:"is_admin"`
	DisabledAt        *time.Time `json:"disabled_at"`
	CreatedAt         time.Time  `json:"created_at"`
	SubscriptionCount int64      `json:"subscription_count"`
	OwnedBlogCount    int64      `json:"owned_blog_count"`
}

func (s *Store) AdminUsers(ctx context.Context, search string, limit, offset int) ([]AdminUser, int64, error) {
	search = strings.TrimSpace(search)
	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE $1 = '' OR email ILIKE '%' || $1 || '%' OR display_name ILIKE '%' || $1 || '%'`, search).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT u.id::text, u.email, u.display_name, u.is_admin, u.disabled_at, u.created_at,
		(SELECT count(*) FROM subscriptions WHERE user_id = u.id),
		(SELECT count(*) FROM blog_owners WHERE user_id = u.id)
		FROM users u WHERE $1 = '' OR u.email ILIKE '%' || $1 || '%' OR u.display_name ILIKE '%' || $1 || '%'
		ORDER BY u.created_at DESC, u.id DESC LIMIT $2 OFFSET $3`, search, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	users, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (AdminUser, error) {
		var u AdminUser
		err := row.Scan(&u.ID, &u.Email, &u.DisplayName, &u.IsAdmin, &u.DisabledAt, &u.CreatedAt, &u.SubscriptionCount, &u.OwnedBlogCount)
		return u, err
	})
	return users, total, err
}

func (s *Store) SetUserDisabled(ctx context.Context, id string, disabled bool) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(1517, 1)`); err != nil {
			return err
		}
		var isAdmin bool
		var wasDisabled bool
		err := tx.QueryRow(ctx, `SELECT is_admin, disabled_at IS NOT NULL FROM users WHERE id::text = $1 FOR UPDATE`, id).Scan(&isAdmin, &wasDisabled)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if isAdmin && disabled && !wasDisabled {
			var activeAdmins int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE is_admin AND disabled_at IS NULL`).Scan(&activeAdmins); err != nil {
				return err
			}
			if activeAdmins <= 1 {
				return ErrConflict
			}
		}
		_, err = tx.Exec(ctx, `UPDATE users SET disabled_at = CASE WHEN $2 THEN now() ELSE NULL END WHERE id::text = $1`, id, disabled)
		if err != nil {
			return err
		}
		if disabled {
			_, err = tx.Exec(ctx, `DELETE FROM sessions WHERE user_id::text = $1`, id)
		}
		return err
	})
}

func (s *Store) SetUserAdminByID(ctx context.Context, id string, admin bool) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(1517, 1)`); err != nil {
			return err
		}
		var current bool
		var disabled bool
		err := tx.QueryRow(ctx, `SELECT is_admin, disabled_at IS NOT NULL FROM users WHERE id::text = $1 FOR UPDATE`, id).Scan(&current, &disabled)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if admin && disabled {
			return ErrConflict
		}
		if current && !admin && !disabled {
			var activeAdmins int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE is_admin AND disabled_at IS NULL`).Scan(&activeAdmins); err != nil {
				return err
			}
			if activeAdmins <= 1 {
				return ErrConflict
			}
		}
		_, err = tx.Exec(ctx, `UPDATE users SET is_admin = $2 WHERE id::text = $1`, id, admin)
		if err != nil {
			return err
		}
		if !admin {
			_, err = tx.Exec(ctx, `DELETE FROM sessions WHERE user_id::text = $1`, id)
		}
		return err
	})
}

type AdminEntry struct {
	ID          int64      `json:"id"`
	BlogHost    string     `json:"blog_host"`
	BlogName    string     `json:"blog_name"`
	Identity    string     `json:"identity"`
	Title       string     `json:"title"`
	URL         string     `json:"url"`
	PublishedAt *time.Time `json:"published_at"`
	SyncedAt    time.Time  `json:"synced_at"`
	Tags        []string   `json:"tags"`
	Hidden      bool       `json:"hidden"`
	HideReason  string     `json:"hide_reason"`
}

func (s *Store) AdminEntries(ctx context.Context, search string, limit, offset int) ([]AdminEntry, int64, error) {
	search = strings.TrimSpace(search)
	filter := `FROM entries e JOIN blogs b ON b.id = e.blog_id
		LEFT JOIN suppressed_entries se ON se.blog_id = e.blog_id AND se.identity = e.identity
		WHERE $1 = '' OR e.title ILIKE '%' || $1 || '%' OR b.host ILIKE '%' || $1 || '%'`
	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) `+filter, search).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT e.id, b.host, b.name, e.identity, e.title, e.url, e.published_at, e.synced_at, e.tags,
		se.blog_id IS NOT NULL, coalesce(se.reason, '') `+filter+`
		ORDER BY e.published_at DESC NULLS LAST, e.id DESC LIMIT $2 OFFSET $3`, search, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	entries, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (AdminEntry, error) {
		var e AdminEntry
		err := row.Scan(&e.ID, &e.BlogHost, &e.BlogName, &e.Identity, &e.Title, &e.URL,
			&e.PublishedAt, &e.SyncedAt, &e.Tags, &e.Hidden, &e.HideReason)
		return e, err
	})
	return entries, total, err
}

func (s *Store) SetEntryHidden(ctx context.Context, id int64, hidden bool, reason, reviewer string) error {
	var blogID int64
	var identity string
	err := s.pool.QueryRow(ctx, `SELECT blog_id, identity FROM entries WHERE id = $1`, id).Scan(&blogID, &identity)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if hidden {
		_, err = s.pool.Exec(ctx, `INSERT INTO suppressed_entries(blog_id, identity, reason, reviewed_by) VALUES ($1, $2, $3, $4)
			ON CONFLICT (blog_id, identity) DO UPDATE SET reason = excluded.reason, reviewed_by = excluded.reviewed_by`, blogID, identity, reason, reviewer)
	} else {
		_, err = s.pool.Exec(ctx, `DELETE FROM suppressed_entries WHERE blog_id = $1 AND identity = $2`, blogID, identity)
	}
	return err
}

type TakedownRequest struct {
	ID            string     `json:"id"`
	TargetType    string     `json:"target_type"`
	BlogHost      string     `json:"blog_host"`
	EntryIdentity *string    `json:"entry_identity"`
	EntryTitle    string     `json:"entry_title"`
	Requester     string     `json:"requester"`
	Reason        string     `json:"reason"`
	Status        string     `json:"status"`
	ReviewNote    string     `json:"review_note"`
	ReviewedBy    string     `json:"reviewed_by"`
	CreatedAt     time.Time  `json:"created_at"`
	ReviewedAt    *time.Time `json:"reviewed_at"`
}

func (s *Store) TakedownRequests(ctx context.Context, status string, limit int) ([]TakedownRequest, error) {
	rows, err := s.pool.Query(ctx, `SELECT t.id::text, t.target_type, b.host, t.entry_identity,
		coalesce(e.title, ''), coalesce(u.email, ''), t.reason, t.status,
		coalesce(t.review_note, ''), coalesce(t.reviewed_by, ''), t.created_at, t.reviewed_at
		FROM takedown_requests t JOIN blogs b ON b.id = t.blog_id
		LEFT JOIN users u ON u.id = t.requester_id
		LEFT JOIN entries e ON e.blog_id = t.blog_id AND e.identity = t.entry_identity
		WHERE t.status = $1 ORDER BY t.created_at DESC LIMIT $2`, status, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (TakedownRequest, error) {
		var r TakedownRequest
		err := row.Scan(&r.ID, &r.TargetType, &r.BlogHost, &r.EntryIdentity, &r.EntryTitle,
			&r.Requester, &r.Reason, &r.Status, &r.ReviewNote, &r.ReviewedBy, &r.CreatedAt, &r.ReviewedAt)
		return r, err
	})
}

func (s *Store) CreateTakedownRequest(ctx context.Context, targetType, host string, entryID int64, requesterID, reason string) error {
	var identity *string
	if targetType == "entry" {
		var value string
		err := s.pool.QueryRow(ctx, `SELECT e.identity FROM entries e JOIN blogs b ON b.id = e.blog_id WHERE b.host = $1 AND e.id = $2`, host, entryID).Scan(&value)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		identity = &value
	}
	var user any
	if requesterID != "" {
		user = requesterID
	}
	tag, err := s.pool.Exec(ctx, `INSERT INTO takedown_requests(target_type, blog_id, entry_identity, requester_id, reason)
		SELECT $1, b.id, $3, $4, $5 FROM blogs b WHERE b.host = $2`, targetType, host, identity, user, reason)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ReviewTakedown(ctx context.Context, id, decision, note, reviewer string) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var target string
		var blogID int64
		var identity *string
		var reason string
		err := tx.QueryRow(ctx, `SELECT target_type, blog_id, entry_identity, reason FROM takedown_requests
			WHERE id::text = $1 AND status = 'pending' FOR UPDATE`, id).Scan(&target, &blogID, &identity, &reason)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotPending
		}
		if err != nil {
			return err
		}
		if decision == "approved" {
			if target == "blog" {
				_, err = tx.Exec(ctx, `UPDATE blogs SET status = 'paused', status_note = $2, updated_at = now() WHERE id = $1`, blogID, reason)
			} else {
				_, err = tx.Exec(ctx, `INSERT INTO suppressed_entries(blog_id, identity, reason, reviewed_by) VALUES ($1, $2, $3, $4)
					ON CONFLICT (blog_id, identity) DO UPDATE SET reason = excluded.reason, reviewed_by = excluded.reviewed_by`, blogID, *identity, reason, reviewer)
			}
			if err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `UPDATE takedown_requests SET status = $2, review_note = $3, reviewed_by = $4, reviewed_at = now() WHERE id::text = $1`, id, decision, note, reviewer)
		return err
	})
}

type SystemSetting struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedBy string    `json:"updated_by"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Store) SystemSettings(ctx context.Context) ([]SystemSetting, error) {
	rows, err := s.pool.Query(ctx, `SELECT key, value, updated_by, updated_at FROM system_settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (SystemSetting, error) {
		var setting SystemSetting
		err := row.Scan(&setting.Key, &setting.Value, &setting.UpdatedBy, &setting.UpdatedAt)
		return setting, err
	})
}

func (s *Store) Setting(ctx context.Context, key string) (string, error) {
	var value string
	err := s.pool.QueryRow(ctx, `SELECT value FROM system_settings WHERE key = $1`, key).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return value, err
}

func (s *Store) SetSetting(ctx context.Context, key, value, reviewer string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE system_settings SET value = $2, updated_by = $3, updated_at = now() WHERE key = $1`, key, value, reviewer)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
