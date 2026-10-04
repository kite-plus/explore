package store

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type AdminUser struct {
	ID                string     `json:"id"`
	Number            int64      `json:"number"`
	Email             string     `json:"email"`
	DisplayName       string     `json:"display_name"`
	IsAdmin           bool       `json:"is_admin"`
	DisabledAt        *time.Time `json:"disabled_at"`
	DisabledReason    string     `json:"disabled_reason"`
	DisabledBy        string     `json:"disabled_by"`
	CreatedAt         time.Time  `json:"created_at"`
	LastSeenAt        *time.Time `json:"last_seen_at"`
	SubscriptionCount int64      `json:"subscription_count"`
	OwnedBlogCount    int64      `json:"owned_blog_count"`
}

const adminUserColumns = `u.id::text, u.number, u.email, u.display_name, u.is_admin, u.disabled_at, u.disabled_reason, u.disabled_by,
	u.created_at, u.last_seen_at,
	(SELECT count(*) FROM subscriptions WHERE user_id = u.id) AS subscription_count,
	(SELECT count(*) FROM blog_owners WHERE user_id = u.id) AS owned_blog_count`

func scanAdminUser(row pgx.Row) (AdminUser, error) {
	var u AdminUser
	err := row.Scan(&u.ID, &u.Number, &u.Email, &u.DisplayName, &u.IsAdmin, &u.DisabledAt, &u.DisabledReason, &u.DisabledBy,
		&u.CreatedAt, &u.LastSeenAt, &u.SubscriptionCount, &u.OwnedBlogCount)
	return u, err
}

// AdminUserQuery filters and orders the admin user list. Empty Status and
// Role match every account; Sort is one of the keys of userSorts.
type AdminUserQuery struct {
	Search string
	Status string // "active" or "disabled"
	Role   string // "admin" or "reader"
	Sort   string
	Asc    bool
	Limit  int
	Offset int
}

// Never-seen accounts count as the least recently seen.
var userSorts = map[string][2]string{
	"created":       {"u.created_at ASC, u.id ASC", "u.created_at DESC, u.id DESC"},
	"number":        {"u.number ASC", "u.number DESC"},
	"seen":          {"u.last_seen_at ASC NULLS FIRST", "u.last_seen_at DESC NULLS LAST"},
	"subscriptions": {"subscription_count ASC", "subscription_count DESC"},
	"blogs":         {"owned_blog_count ASC", "owned_blog_count DESC"},
}

// ValidUserSort reports whether sort names a column the list can order by.
func ValidUserSort(sort string) bool {
	_, ok := userSorts[sort]
	return ok
}

// AdminUserCounts are the facet counts next to the list's filters. Each
// filter's counts apply the search and the other filter, not itself.
type AdminUserCounts struct {
	Active   int64 `json:"active"`
	Disabled int64 `json:"disabled"`
	Admin    int64 `json:"admin"`
	Reader   int64 `json:"reader"`
}

const (
	// "12" or "#12" finds the account whose ID (number) is 12, besides any text match.
	userSearch = `($1 = '' OR u.email ILIKE '%' || $1 || '%' OR u.display_name ILIKE '%' || $1 || '%'
		OR u.id::text = $1 OR u.number::text = ltrim($1, '#'))`
	userStatus = `($2 = '' OR (u.disabled_at IS NOT NULL) = ($2 = 'disabled'))`
	userRole   = `($3 = '' OR u.is_admin = ($3 = 'admin'))`
)

func (s *Store) AdminUsers(ctx context.Context, q AdminUserQuery) ([]AdminUser, int64, AdminUserCounts, error) {
	search := strings.TrimSpace(q.Search)
	var total int64
	var counts AdminUserCounts
	err := s.pool.QueryRow(ctx, `SELECT
		count(*) FILTER (WHERE `+userStatus+` AND `+userRole+`),
		count(*) FILTER (WHERE u.disabled_at IS NULL AND `+userRole+`),
		count(*) FILTER (WHERE u.disabled_at IS NOT NULL AND `+userRole+`),
		count(*) FILTER (WHERE u.is_admin AND `+userStatus+`),
		count(*) FILTER (WHERE NOT u.is_admin AND `+userStatus+`)
		FROM users u WHERE `+userSearch, search, q.Status, q.Role).Scan(
		&total, &counts.Active, &counts.Disabled, &counts.Admin, &counts.Reader)
	if err != nil {
		return nil, 0, counts, err
	}
	sort, ok := userSorts[q.Sort]
	if !ok {
		sort = userSorts["created"]
	}
	order := sort[1]
	if q.Asc {
		order = sort[0]
	}
	if q.Sort != "created" && q.Sort != "number" {
		order += ", u.created_at DESC, u.id DESC"
	}
	rows, err := s.pool.Query(ctx, `SELECT `+adminUserColumns+` FROM users u
		WHERE `+userSearch+` AND `+userStatus+` AND `+userRole+`
		ORDER BY `+order+` LIMIT $4 OFFSET $5`, search, q.Status, q.Role, q.Limit, q.Offset)
	if err != nil {
		return nil, 0, counts, err
	}
	users, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (AdminUser, error) { return scanAdminUser(row) })
	return users, total, counts, err
}

type AdminUserSession struct {
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// AdminUserBlog is a blog an account follows or owns; Since is when it
// followed it or proved it owns it.
type AdminUserBlog struct {
	Host   string    `json:"host"`
	Name   string    `json:"name"`
	Status string    `json:"status"`
	Since  time.Time `json:"since"`
}

type AdminUserClaim struct {
	Host      string    `json:"host"`
	ExpiresAt time.Time `json:"expires_at"`
}

type AdminUserReport struct {
	ID         string    `json:"id"`
	TargetType string    `json:"target_type"`
	BlogHost   string    `json:"blog_host"`
	EntryTitle string    `json:"entry_title"`
	Reason     string    `json:"reason"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

// AdminUserDetail is one account with what hangs off it. Subscriptions and
// reports are capped at the newest DetailListLimit; the counts are not.
type AdminUserDetail struct {
	AdminUser
	Sessions      []AdminUserSession `json:"sessions"`
	Subscriptions []AdminUserBlog    `json:"subscriptions"`
	OwnedBlogs    []AdminUserBlog    `json:"owned_blogs"`
	PendingClaims []AdminUserClaim   `json:"pending_claims"`
	Reports       []AdminUserReport  `json:"reports"`
}

const DetailListLimit = 200

func (s *Store) AdminUserDetail(ctx context.Context, id string) (AdminUserDetail, error) {
	var d AdminUserDetail
	err := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead}, func(tx pgx.Tx) error {
		u, err := scanAdminUser(tx.QueryRow(ctx, `SELECT `+adminUserColumns+` FROM users u WHERE u.id::text = $1`, id))
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		d.AdminUser = u
		if d.Sessions, err = collectByID(ctx, tx, `SELECT created_at, expires_at FROM sessions
			WHERE user_id = $1 AND expires_at > now() ORDER BY created_at DESC`, u.ID,
			func(row pgx.CollectableRow) (AdminUserSession, error) {
				var v AdminUserSession
				return v, row.Scan(&v.CreatedAt, &v.ExpiresAt)
			}); err != nil {
			return err
		}
		scanBlog := func(row pgx.CollectableRow) (AdminUserBlog, error) {
			var v AdminUserBlog
			return v, row.Scan(&v.Host, &v.Name, &v.Status, &v.Since)
		}
		if d.Subscriptions, err = collectByID(ctx, tx, `SELECT b.host, b.name, b.status, s.created_at
			FROM subscriptions s JOIN blogs b ON b.id = s.blog_id
			WHERE s.user_id = $1 ORDER BY s.created_at DESC, b.host LIMIT `+strconv.Itoa(DetailListLimit), u.ID, scanBlog); err != nil {
			return err
		}
		if d.OwnedBlogs, err = collectByID(ctx, tx, `SELECT b.host, b.name, b.status, o.verified_at
			FROM blog_owners o JOIN blogs b ON b.id = o.blog_id
			WHERE o.user_id = $1 ORDER BY o.verified_at DESC, b.host`, u.ID, scanBlog); err != nil {
			return err
		}
		if d.PendingClaims, err = collectByID(ctx, tx, `SELECT b.host, c.expires_at
			FROM blog_claim_challenges c JOIN blogs b ON b.id = c.blog_id
			WHERE c.user_id = $1 AND c.expires_at > now() ORDER BY c.expires_at DESC`, u.ID,
			func(row pgx.CollectableRow) (AdminUserClaim, error) {
				var v AdminUserClaim
				return v, row.Scan(&v.Host, &v.ExpiresAt)
			}); err != nil {
			return err
		}
		d.Reports, err = collectByID(ctx, tx, `SELECT t.id::text, t.target_type, b.host, coalesce(e.title, ''), t.reason, t.status, t.created_at
			FROM takedown_requests t JOIN blogs b ON b.id = t.blog_id
			LEFT JOIN entries e ON e.blog_id = t.blog_id AND e.identity = t.entry_identity
			WHERE t.requester_id = $1 ORDER BY t.created_at DESC LIMIT `+strconv.Itoa(DetailListLimit), u.ID,
			func(row pgx.CollectableRow) (AdminUserReport, error) {
				var v AdminUserReport
				return v, row.Scan(&v.ID, &v.TargetType, &v.BlogHost, &v.EntryTitle, &v.Reason, &v.Status, &v.CreatedAt)
			})
		return err
	})
	return d, err
}

func collectByID[T any](ctx context.Context, tx pgx.Tx, sql, id string, scan func(pgx.CollectableRow) (T, error)) ([]T, error) {
	rows, err := tx.Query(ctx, sql, id)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scan)
}

// lockUser takes the lock every change of an account's admin standing holds,
// so two changes cannot each leave the other admin as the last one, and
// returns whether the account is an admin and whether it is disabled.
func lockUser(ctx context.Context, tx pgx.Tx, id string) (admin, disabled bool, err error) {
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(1517, 1)`); err != nil {
		return false, false, err
	}
	err = tx.QueryRow(ctx, `SELECT is_admin, disabled_at IS NOT NULL FROM users WHERE id::text = $1 FOR UPDATE`, id).Scan(&admin, &disabled)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return admin, disabled, err
}

func lastActiveAdmin(ctx context.Context, tx pgx.Tx) (bool, error) {
	var n int
	err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE is_admin AND disabled_at IS NULL`).Scan(&n)
	return n <= 1, err
}

// SetUserDisabled disables an account, recording why and by whom, and ends
// its sessions; or restores it, clearing both.
func (s *Store) SetUserDisabled(ctx context.Context, id string, disabled bool, reason, by string) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		isAdmin, wasDisabled, err := lockUser(ctx, tx, id)
		if err != nil {
			return err
		}
		if isAdmin && disabled && !wasDisabled {
			last, err := lastActiveAdmin(ctx, tx)
			if err != nil {
				return err
			}
			if last {
				return ErrLastAdmin
			}
		}
		if disabled {
			// Disabling again keeps the original time but takes the new reason.
			_, err = tx.Exec(ctx, `UPDATE users SET disabled_at = coalesce(disabled_at, now()), disabled_reason = $2, disabled_by = $3
				WHERE id::text = $1`, id, reason, by)
			if err == nil {
				_, err = tx.Exec(ctx, `DELETE FROM sessions WHERE user_id::text = $1`, id)
			}
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE users SET disabled_at = NULL, disabled_reason = '', disabled_by = '' WHERE id::text = $1`, id)
		return err
	})
}

func (s *Store) SetUserAdminByID(ctx context.Context, id string, admin bool) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		current, disabled, err := lockUser(ctx, tx, id)
		if err != nil {
			return err
		}
		if admin && disabled {
			return ErrUserDisabled
		}
		if current && !admin && !disabled {
			last, err := lastActiveAdmin(ctx, tx)
			if err != nil {
				return err
			}
			if last {
				return ErrLastAdmin
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

func (s *Store) RenameUser(ctx context.Context, id, displayName string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE users SET display_name = $2 WHERE id::text = $1`, id, displayName)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RevokeSessions signs an account out everywhere and returns how many live
// sessions it ended.
func (s *Store) RevokeSessions(ctx context.Context, id string) (int64, error) {
	var exists bool
	var revoked int64
	err := s.pool.QueryRow(ctx, `WITH gone AS (
			DELETE FROM sessions WHERE user_id::text = $1 RETURNING expires_at
		)
		SELECT EXISTS (SELECT 1 FROM users WHERE id::text = $1),
			(SELECT count(*) FROM gone WHERE expires_at > now())`, id).Scan(&exists, &revoked)
	if err == nil && !exists {
		err = ErrNotFound
	}
	return revoked, err
}

// ReleaseBlog removes an account's proven ownership of a blog, so the blog
// can be claimed again.
func (s *Store) ReleaseBlog(ctx context.Context, id, host string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM blog_owners o USING blogs b
		WHERE o.blog_id = b.id AND b.host = $2 AND o.user_id::text = $1`, id, host)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
