package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/kite-plus/explore/internal/policy"
)

var (
	ErrConflict = errors.New("conflict")
	// ErrLastAdmin refuses a change that would leave no active admin.
	ErrLastAdmin = errors.New("last active admin")
	// ErrUserDisabled refuses admin access for a disabled account.
	ErrUserDisabled = errors.New("account is disabled")
)

type User struct {
	ID           string
	Number       int64
	Email        string
	PasswordHash string
	DisplayName  string
	IsAdmin      bool
	Disabled     bool
	CreatedAt    time.Time
	// TemporaryPassword is set while an admin-reset password is unchanged.
	TemporaryPassword bool
}

// userColumns are what scanUser reads, in order.
const userColumns = `id::text, number, email, password_hash, display_name, is_admin, disabled_at IS NOT NULL, created_at,
	password_reset_at IS NOT NULL`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Number, &u.Email, &u.PasswordHash, &u.DisplayName, &u.IsAdmin, &u.Disabled, &u.CreatedAt, &u.TemporaryPassword)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// CreateUser checks the email before inserting, so a taken email does not
// use up a member number; only a race between two sign-ups still can.
func (s *Store) CreateUser(ctx context.Context, email, passwordHash, displayName string) (User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, `INSERT INTO users(email, password_hash, display_name)
		SELECT $1, $2, $3 WHERE NOT EXISTS (SELECT 1 FROM users WHERE email = $1)
		RETURNING `+userColumns, email, passwordHash, displayName))
	if errors.Is(err, ErrNotFound) || isUniqueViolation(err) {
		return User{}, ErrConflict
	}
	return u, err
}

func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE email = $1`, email))
}

// UserBySession also marks the account as seen, at most once every five
// minutes, so most requests write nothing.
func (s *Store) UserBySession(ctx context.Context, tokenHash [sha256.Size]byte) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `WITH found AS (
			SELECT u.id, u.number, u.email, u.password_hash, u.display_name, u.is_admin, u.created_at, u.last_seen_at, u.password_reset_at
			FROM sessions s JOIN users u ON u.id = s.user_id
			WHERE s.token_hash = $1 AND s.expires_at > now() AND u.disabled_at IS NULL
		), seen AS (
			UPDATE users SET last_seen_at = now() FROM found
			WHERE users.id = found.id AND (found.last_seen_at IS NULL OR found.last_seen_at < now() - interval '5 minutes')
		)
		SELECT id::text, number, email, password_hash, display_name, is_admin, false, created_at, password_reset_at IS NOT NULL FROM found`, tokenHash[:]))
}

func (s *Store) CreateSession(ctx context.Context, userID string, tokenHash [sha256.Size]byte, expires time.Time) error {
	_, err := s.pool.Exec(ctx, `WITH created AS (INSERT INTO sessions(token_hash, user_id, expires_at) VALUES ($1, $2, $3))
		UPDATE users SET last_seen_at = now() WHERE id = $2`, tokenHash[:], userID, expires)
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

// SetPassword replaces an account's password hash and ends every session of
// it but the one with the token hash keep, so a changed password locks out
// whoever else was signed in with the old one. It also ends a temporary
// password an admin set.
func (s *Store) SetPassword(ctx context.Context, userID, passwordHash string, keep [sha256.Size]byte) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE users SET password_hash = $2, password_reset_at = NULL, password_reset_by = ''
			WHERE id::text = $1`, userID, passwordHash)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		_, err = tx.Exec(ctx, `DELETE FROM sessions WHERE user_id::text = $1 AND token_hash <> $2`, userID, keep[:])
		return err
	})
}

// DeleteUser deletes an account with its sessions, follows and claims; its
// reports stay, with no requester. The last active admin gets ErrLastAdmin,
// since the site would be left with no admin.
func (s *Store) DeleteUser(ctx context.Context, userID string) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		isAdmin, disabled, err := lockUser(ctx, tx, userID)
		if err != nil {
			return err
		}
		if isAdmin && !disabled {
			last, err := lastActiveAdmin(ctx, tx)
			if err != nil {
				return err
			}
			if last {
				return ErrLastAdmin
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
	// As in Stream, the page is picked before what readers see is worked out.
	rows, err := s.pool.Query(ctx, `WITH followed AS (
		SELECT e.id, CASE WHEN e.date_trusted THEN e.published_at ELSE 'epoch'::timestamptz END AS sort_at
		FROM subscriptions sub JOIN blogs b ON b.id = sub.blog_id JOIN entries e ON e.blog_id = b.id
		WHERE sub.user_id = @user_id AND `+visible+`
		AND `+notSuppressed+`
		AND (e.published_at IS NULL OR e.published_at <= now() + (@future_tolerance * interval '1 second'))
		AND (@lang::text = '' OR lower(b.language) = @lang OR lower(b.language) LIKE @lang || '-%')
		AND (@tag::text = '' OR @tag = ANY(e.tags))
		), page AS (
		SELECT id, sort_at FROM followed WHERE NOT @has_cursor OR (sort_at, id) < (@cursor_at, @cursor_id)
		ORDER BY sort_at DESC, id DESC LIMIT @limit
		)
		SELECT e.id, e.blog_id, e.identity, e.url, e.title, coalesce(`+shownExcerpt+`, ''), coalesce(`+shownImage+`, ''),
			CASE WHEN e.date_trusted THEN e.published_at END,
			e.tags, e.link_status, e.link_checked_at, b.host, b.name, b.site_url, b.feed_url, b.language, p.sort_at
		FROM page p JOIN entries e ON e.id = p.id JOIN blogs b ON b.id = e.blog_id
		ORDER BY p.sort_at DESC, p.id DESC`, args)
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

// FeedToken is the token of the address that publishes the user's following
// stream, or "" while they keep it off.
func (s *Store) FeedToken(ctx context.Context, userID string) (string, error) {
	var token *string
	err := s.pool.QueryRow(ctx, `SELECT feed_token FROM users WHERE id = $1`, userID).Scan(&token)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil || token == nil {
		return "", err
	}
	return *token, nil
}

// SetFeedToken publishes the user's following stream under token, in place of
// the token before, or stops publishing it when token is "". The token is
// kept as it is rather than hashed: its owner reads the address back, and
// it reveals no more than the follows stored beside it.
func (s *Store) SetFeedToken(ctx context.Context, userID, token string) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET feed_token = nullif($2, '') WHERE id = $1`, userID, token)
	return err
}

// FeedOwner is the account whose following stream is published under token.
// A token no one uses, and one of a disabled account, are ErrNotFound.
func (s *Store) FeedOwner(ctx context.Context, token string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT id::text FROM users WHERE feed_token = $1 AND disabled_at IS NULL`, token).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return id, err
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

// ImportItem is one feed from an imported OPML file: the hosts its
// addresses name, and its feed address in the form blogs store.
type ImportItem struct {
	Hosts   []string
	FeedURL string
}

// Imported is the outcome of an import.
type Imported struct {
	Matched []bool // by item: whether it named a visible blog
	Added   int    // blogs newly followed
	Already int    // matched blogs that were followed before
}

// ImportSubscriptions follows every visible blog the items name, by host,
// extra domain or exact feed address. It never lists a blog.
func (s *Store) ImportSubscriptions(ctx context.Context, userID string, items []ImportItem) (Imported, error) {
	var hostItems, feedItems []int32
	var hosts, feeds []string
	for i, it := range items {
		for _, h := range it.Hosts {
			hostItems, hosts = append(hostItems, int32(i)), append(hosts, h)
		}
		if it.FeedURL != "" {
			feedItems, feeds = append(feedItems, int32(i)), append(feeds, it.FeedURL)
		}
	}
	var matched []int32
	var blogs, added int
	err := s.pool.QueryRow(ctx, `
		WITH hosts AS (
			SELECT * FROM unnest(@host_items::int[], @hosts::text[]) AS h (item, host)
		), candidates AS (
			SELECT h.item, b.id FROM hosts h JOIN blogs b ON b.host = h.host
			UNION
			SELECT h.item, b.id FROM blogs b CROSS JOIN LATERAL unnest(b.extra_domains) AS d (domain)
			JOIN hosts h ON h.host = d.domain
			UNION
			SELECT f.item, b.id FROM unnest(@feed_items::int[], @feeds::text[]) AS f (item, feed_url)
			JOIN blogs b ON b.feed_url = f.feed_url
		), matched AS (
			SELECT c.item, c.id FROM candidates c JOIN blogs b ON b.id = c.id WHERE `+visible+`
		), added AS (
			INSERT INTO subscriptions (user_id, blog_id)
			SELECT DISTINCT @user_id::uuid, id FROM matched
			ON CONFLICT DO NOTHING
			RETURNING blog_id
		)
		SELECT coalesce((SELECT array_agg(DISTINCT item) FROM matched), '{}'),
		       (SELECT count(DISTINCT id) FROM matched), (SELECT count(*) FROM added)`,
		pgx.NamedArgs{
			"user_id": userID, "host_items": hostItems, "hosts": hosts, "feed_items": feedItems, "feeds": feeds,
			"unhealthy_after": seconds(policy.UnhealthyAfter),
		}).Scan(&matched, &blogs, &added)
	if err != nil {
		return Imported{}, err
	}
	out := Imported{Matched: make([]bool, len(items)), Added: added, Already: blogs - added}
	for _, i := range matched {
		out.Matched[i] = true
	}
	return out, nil
}
