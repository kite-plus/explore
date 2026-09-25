package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/kite-plus/explore/internal/policy"
)

var ErrConflict = errors.New("conflict")

type User struct {
	ID           string
	Email        string
	PasswordHash string
	DisplayName  string
	IsAdmin      bool
	Disabled     bool
}

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.IsAdmin, &u.Disabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *Store) CreateUser(ctx context.Context, email, passwordHash, displayName string) (User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, `INSERT INTO users(email, password_hash, display_name)
		VALUES ($1, $2, $3) RETURNING id::text, email, password_hash, display_name, is_admin, disabled_at IS NOT NULL`, email, passwordHash, displayName))
	if isUniqueViolation(err) {
		return User{}, ErrConflict
	}
	return u, err
}

func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `SELECT id::text, email, password_hash, display_name, is_admin, disabled_at IS NOT NULL FROM users WHERE email = $1`, email))
}

func (s *Store) UserBySession(ctx context.Context, tokenHash [sha256.Size]byte) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `SELECT u.id::text, u.email, u.password_hash, u.display_name, u.is_admin, u.disabled_at IS NOT NULL
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now() AND u.disabled_at IS NULL`, tokenHash[:]))
}

func (s *Store) CreateSession(ctx context.Context, userID string, tokenHash [sha256.Size]byte, expires time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO sessions(token_hash, user_id, expires_at) VALUES ($1, $2, $3)`, tokenHash[:], userID, expires)
	return err
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash [sha256.Size]byte) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash[:])
	return err
}

func (s *Store) UpdateUserName(ctx context.Context, userID, displayName string) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET display_name = $2 WHERE id = $1`, userID, displayName)
	return err
}

// DeleteUser deletes an account with its sessions, follows and claims; its
// reports stay, with no requester. The last active admin gets ErrConflict,
// since the site would be left with no admin.
func (s *Store) DeleteUser(ctx context.Context, userID string) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(1517, 1)`); err != nil {
			return err
		}
		var isAdmin, disabled bool
		err := tx.QueryRow(ctx, `SELECT is_admin, disabled_at IS NOT NULL FROM users WHERE id::text = $1 FOR UPDATE`, userID).Scan(&isAdmin, &disabled)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if isAdmin && !disabled {
			var activeAdmins int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE is_admin AND disabled_at IS NULL`).Scan(&activeAdmins); err != nil {
				return err
			}
			if activeAdmins <= 1 {
				return ErrConflict
			}
		}
		_, err = tx.Exec(ctx, `DELETE FROM users WHERE id::text = $1`, userID)
		return err
	})
}

func (s *Store) SetUserAdmin(ctx context.Context, email string, admin bool) error {
	tag, err := s.pool.Exec(ctx, `UPDATE users SET is_admin = $2 WHERE email = $1`, email, admin)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) AddSubscription(ctx context.Context, userID, host string) error {
	tag, err := s.pool.Exec(ctx, `INSERT INTO subscriptions(user_id, blog_id)
		SELECT $1, b.id FROM blogs b WHERE b.host = $2 AND b.status = 'active' AND b.gone_since IS NULL
		AND b.last_succeeded_at > now() - ($3 * interval '1 second')
		ON CONFLICT DO NOTHING`, userID, host, seconds(policy.UnhealthyAfter))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		err = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM subscriptions s JOIN blogs b ON b.id = s.blog_id WHERE s.user_id = $1 AND b.host = $2)`, userID, host).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
	}
	return nil
}

func (s *Store) RemoveSubscription(ctx context.Context, userID, host string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM subscriptions s USING blogs b WHERE s.blog_id = b.id AND s.user_id = $1 AND b.host = $2`, userID, host)
	return err
}

func (s *Store) Subscriptions(ctx context.Context, userID string) ([]ListedBlog, error) {
	rows, err := s.pool.Query(ctx, `SELECT b.id, b.host, b.name, b.description, b.site_url, b.feed_url, b.language, b.generator,
		(SELECT max(e.published_at) FROM entries e WHERE e.blog_id = b.id AND e.date_trusted
		 AND `+notSuppressed+`) AS last_published_at
		FROM subscriptions s JOIN blogs b ON b.id = s.blog_id
		WHERE s.user_id = $1 ORDER BY s.created_at DESC, b.id DESC`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanListed)
}

func (s *Store) FollowingStream(ctx context.Context, userID string, q StreamQuery) ([]StreamEntry, error) {
	args := pgx.NamedArgs{"user_id": userID, "lang": q.Lang, "tag": q.Tag, "limit": q.Limit,
		"has_cursor": q.Cursor != nil, "cursor_at": time.Time{}, "cursor_id": int64(0),
		"unhealthy_after": seconds(policy.UnhealthyAfter), "future_tolerance": seconds(policy.FutureTolerance)}
	if q.Cursor != nil {
		args["cursor_at"], args["cursor_id"] = q.Cursor.At, q.Cursor.ID
	}
	rows, err := s.pool.Query(ctx, `WITH followed AS (
		SELECT e.id, e.blog_id, e.identity, e.url, e.title, coalesce(`+shownExcerpt+`, '') AS excerpt,
			coalesce(`+shownImage+`, '') AS image_url,
			CASE WHEN e.date_trusted THEN e.published_at END AS published_at,
			CASE WHEN e.date_trusted THEN e.published_at ELSE 'epoch'::timestamptz END AS sort_at,
			e.tags, e.link_status, e.link_checked_at, b.host, b.name, b.site_url, b.feed_url, b.language
		FROM subscriptions sub JOIN blogs b ON b.id = sub.blog_id JOIN entries e ON e.blog_id = b.id
		WHERE sub.user_id = @user_id AND `+visible+`
		AND `+notSuppressed+`
		AND (e.published_at IS NULL OR e.published_at <= now() + (@future_tolerance * interval '1 second'))
		AND (@lang::text = '' OR lower(b.language) = @lang OR lower(b.language) LIKE @lang || '-%')
		AND (@tag::text = '' OR @tag = ANY(e.tags))
		)
		SELECT id, blog_id, identity, url, title, excerpt, image_url, published_at, tags, link_status,
			link_checked_at, host, name, site_url, feed_url, language, sort_at
		FROM followed WHERE NOT @has_cursor OR (sort_at, id) < (@cursor_at, @cursor_id)
		ORDER BY sort_at DESC, id DESC LIMIT @limit`, args)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (StreamEntry, error) {
		var e StreamEntry
		err := row.Scan(&e.ID, &e.BlogID, &e.Identity, &e.URL, &e.Title, &e.Excerpt, &e.ImageURL, &e.PublishedAt,
			&e.Tags, &e.LinkStatus, &e.LinkCheckedAt, &e.Blog.Host, &e.Blog.Name, &e.Blog.SiteURL, &e.Blog.FeedURL, &e.Blog.Language, &e.SortAt)
		e.DateTrusted = e.PublishedAt != nil
		return e, err
	})
}

func (s *Store) CreateBlogClaim(ctx context.Context, userID, host string, tokenHash [sha256.Size]byte, expires time.Time) error {
	var ownerID string
	err := s.pool.QueryRow(ctx, `SELECT coalesce(o.user_id::text, '') FROM blogs b LEFT JOIN blog_owners o ON o.blog_id = b.id WHERE b.host = $1`, host).Scan(&ownerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if ownerID != "" {
		return ErrConflict
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO blog_claim_challenges(blog_id, user_id, token_hash, expires_at)
		SELECT id, $2, $3, $4 FROM blogs WHERE host = $1
		ON CONFLICT (blog_id, user_id) DO UPDATE SET token_hash = excluded.token_hash, expires_at = excluded.expires_at`, host, userID, tokenHash[:], expires)
	return err
}

func (s *Store) ConfirmBlogClaim(ctx context.Context, userID, host string, tokenHash [sha256.Size]byte) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	// After Commit this only reports the closed transaction.
	defer func() { _ = tx.Rollback(ctx) }()
	var blogID int64
	err = tx.QueryRow(ctx, `SELECT c.blog_id FROM blog_claim_challenges c JOIN blogs b ON b.id = c.blog_id
		WHERE c.user_id = $1 AND b.host = $2 AND c.token_hash = $3 AND c.expires_at > now() FOR UPDATE OF c`, userID, host, tokenHash[:]).Scan(&blogID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO blog_owners(blog_id, user_id) VALUES ($1, $2)`, blogID, userID)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `DELETE FROM blog_claim_challenges WHERE blog_id = $1`, blogID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) BlogClaimHash(ctx context.Context, userID, host string) ([sha256.Size]byte, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT c.token_hash FROM blog_claim_challenges c JOIN blogs b ON b.id = c.blog_id
		WHERE c.user_id = $1 AND b.host = $2 AND c.expires_at > now()`, userID, host).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return [sha256.Size]byte{}, ErrNotFound
	}
	var hash [sha256.Size]byte
	copy(hash[:], raw)
	return hash, err
}

func (s *Store) OwnedBlogs(ctx context.Context, userID string) ([]ListedBlog, error) {
	rows, err := s.pool.Query(ctx, `SELECT b.id, b.host, b.name, b.description, b.site_url, b.feed_url, b.language, b.generator,
		(SELECT max(e.published_at) FROM entries e WHERE e.blog_id = b.id AND e.date_trusted
		 AND `+notSuppressed+`)
		FROM blog_owners o JOIN blogs b ON b.id = o.blog_id WHERE o.user_id = $1 ORDER BY b.host`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanListed)
}
