package store

import (
	"cmp"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite-plus/explore/internal/model"
)

// Session clients; see docs/design/accounts.md section 1.1.
const (
	ClientWeb     = "web"
	ClientIOS     = "ios"
	ClientAndroid = "android"
)

var (
	// ErrIdentityTaken refuses linking an identity another account has.
	ErrIdentityTaken = errors.New("identity is linked to another account")
	// ErrAlreadyLinked refuses linking a second identity of one issuer.
	ErrAlreadyLinked = errors.New("account is linked to another identity")
	// ErrPasswordOff refuses a password for an account that signs in through
	// its identity provider.
	ErrPasswordOff = errors.New("account signs in through its identity provider")
	// ErrInvalidGrant refuses an app code that is unknown, used, expired or
	// does not match the verifier.
	ErrInvalidGrant = errors.New("invalid grant")
)

// Session is a session to create. SID is the identity provider's session it
// came from, empty for a password sign-in; Client defaults to ClientWeb.
type Session struct {
	UserID    string
	TokenHash [sha256.Size]byte
	Expires   time.Time
	SID       string
	Client    string
}

func (s *Store) CreateSession(ctx context.Context, sess Session) error {
	return createSession(ctx, s.pool, sess)
}

// execer is a pool or a transaction.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func createSession(ctx context.Context, db execer, sess Session) error {
	_, err := db.Exec(ctx, `WITH created AS (
			INSERT INTO sessions (token_hash, user_id, expires_at, sid, client) VALUES ($1, $2, $3, nullif($4, ''), $5)
		)
		UPDATE users SET last_seen_at = now() WHERE id = $2`,
		sess.TokenHash[:], sess.UserID, sess.Expires, sess.SID, cmp.Or(sess.Client, ClientWeb))
	return err
}

// UserByIdentity is the account linked to the identity (issuer, subject).
func (s *Store) UserByIdentity(ctx context.Context, issuer, subject string) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users
		WHERE id = (SELECT user_id FROM user_identities WHERE issuer = $1 AND subject = $2)`, issuer, subject))
}

// CreateIdentityUser creates an account with no password for a new
// identity. An email a local account already uses is ErrEmailTaken, since
// accounts are never merged by email; an identity another sign-in has just
// created an account for is ErrIdentityTaken.
func (s *Store) CreateIdentityUser(ctx context.Context, issuer, subject, email, displayName string) (User, error) {
	var u User
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		u, err = scanUser(tx.QueryRow(ctx, `INSERT INTO users (email, password_hash, display_name)
			SELECT $1, '', $2 WHERE NOT EXISTS (SELECT 1 FROM users WHERE email = $1)
			RETURNING `+userColumns, email, displayName))
		if errors.Is(err, ErrNotFound) || isUniqueViolation(err) {
			return ErrEmailTaken
		}
		if err != nil {
			return err
		}
		var linked time.Time
		err = tx.QueryRow(ctx, `INSERT INTO user_identities (issuer, subject, user_id) VALUES ($1, $2, $3) RETURNING linked_at`,
			issuer, subject, u.ID).Scan(&linked)
		if isUniqueViolation(err) {
			return ErrIdentityTaken
		}
		u.LinkedAt = &linked
		return err
	})
	return u, err
}

// LinkIdentity attaches the identity (issuer, subject) to an account and
// turns off its password, which the identity provider replaces. Linking the
// identity the account already has changes nothing.
func (s *Store) LinkIdentity(ctx context.Context, userID, issuer, subject string) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var disabled bool
		err := tx.QueryRow(ctx, `SELECT disabled_at IS NOT NULL FROM users WHERE id::text = $1 FOR UPDATE`, userID).Scan(&disabled)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if disabled {
			return ErrUserDisabled
		}
		var owner string
		err = tx.QueryRow(ctx, `SELECT user_id::text FROM user_identities WHERE issuer = $1 AND subject = $2`, issuer, subject).Scan(&owner)
		switch {
		case err == nil && owner == userID:
			return nil
		case err == nil:
			return ErrIdentityTaken
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}
		var other bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM user_identities WHERE user_id::text = $1 AND issuer = $2)`,
			userID, issuer).Scan(&other); err != nil {
			return err
		}
		if other {
			return ErrAlreadyLinked
		}
		if _, err := tx.Exec(ctx, `INSERT INTO user_identities (issuer, subject, user_id) VALUES ($1, $2, $3)`, issuer, subject, userID); err != nil {
			if isUniqueViolation(err) {
				return ErrIdentityTaken
			}
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE users SET password_hash = '', password_reset_at = NULL, password_reset_by = '' WHERE id::text = $1`, userID)
		return err
	})
}

// EndIdentitySessions deletes the sessions a back-channel logout names: with
// a sid, those signed in through that provider session (of the subject's
// account, when a subject is given too); with a subject alone, every session
// of its account. It returns how many it deleted.
func (s *Store) EndIdentitySessions(ctx context.Context, issuer, subject, sid string) (int64, error) {
	if sid == "" && subject == "" {
		return 0, nil
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM sessions s USING user_identities i
		WHERE i.user_id = s.user_id AND i.issuer = $1
		AND ($2 = '' OR i.subject = $2) AND ($3 = '' OR s.sid = $3)`, issuer, subject, sid)
	return tag.RowsAffected(), err
}

// LoginFlow is a sign-in waiting for the provider to send the reader back.
// Mode is "web", "link" (UserID links an identity) or "app" (Challenge,
// Client and AppRedirect describe the app sign-in).
type LoginFlow struct {
	StateHash   [sha256.Size]byte
	BrowserHash [sha256.Size]byte
	Mode        string
	Nonce       string
	Verifier    string
	ReturnPath  string
	UserID      string
	Challenge   string
	Client      string
	AppRedirect string
	Expires     time.Time
}

func (s *Store) CreateLoginFlow(ctx context.Context, f LoginFlow) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO login_flows (state_hash, browser_hash, mode, nonce, verifier, return_path,
			user_id, challenge, client, app_redirect, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, nullif($7, '')::uuid, nullif($8, ''), $9, $10, $11)`,
		f.StateHash[:], f.BrowserHash[:], f.Mode, f.Nonce, f.Verifier, f.ReturnPath, f.UserID, f.Challenge,
		cmp.Or(f.Client, ClientWeb), f.AppRedirect, f.Expires)
	return err
}

// TakeLoginFlow removes and returns the flow with this state, if the same
// browser started it; one that has expired is ErrNotFound.
func (s *Store) TakeLoginFlow(ctx context.Context, stateHash, browserHash [sha256.Size]byte) (LoginFlow, error) {
	f := LoginFlow{StateHash: stateHash, BrowserHash: browserHash}
	var userID, challenge *string
	err := s.pool.QueryRow(ctx, `DELETE FROM login_flows WHERE state_hash = $1 AND browser_hash = $2
		RETURNING mode, nonce, verifier, return_path, user_id::text, challenge, client, app_redirect, expires_at`,
		stateHash[:], browserHash[:]).Scan(&f.Mode, &f.Nonce, &f.Verifier, &f.ReturnPath, &userID, &challenge, &f.Client, &f.AppRedirect, &f.Expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return LoginFlow{}, ErrNotFound
	}
	if err != nil {
		return LoginFlow{}, err
	}
	if !f.Expires.After(time.Now()) {
		return LoginFlow{}, ErrNotFound
	}
	f.UserID, f.Challenge = deref(userID), deref(challenge)
	return f, nil
}

// AppCode is a one-time code an app exchanges for a session.
type AppCode struct {
	CodeHash  [sha256.Size]byte
	UserID    string
	SID       string
	Challenge string
	Client    string
	Expires   time.Time
}

func (s *Store) CreateAppCode(ctx context.Context, c AppCode) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO app_codes (code_hash, user_id, sid, challenge, client, expires_at)
		VALUES ($1, $2, nullif($3, ''), $4, $5, $6)`, c.CodeHash[:], c.UserID, c.SID, c.Challenge, c.Client, c.Expires)
	return err
}

// RedeemAppCode uses up an app code and, when it had not expired or been
// used and verify accepts its challenge, creates a session with tokenHash
// for its account, all in one transaction. Anything else is ErrInvalidGrant;
// a code tried again also ends the session it was first exchanged for.
func (s *Store) RedeemAppCode(ctx context.Context, codeHash [sha256.Size]byte, verify func(challenge string) bool,
	tokenHash [sha256.Size]byte, expires time.Time) (User, error) {
	var u User
	granted := false
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var c AppCode
		var sid *string
		var expiresAt time.Time
		var usedAt *time.Time
		var sessionHash []byte
		err := tx.QueryRow(ctx, `SELECT user_id::text, sid, challenge, client, expires_at, used_at, session_hash
			FROM app_codes WHERE code_hash = $1 FOR UPDATE`, codeHash[:]).Scan(&c.UserID, &sid, &c.Challenge, &c.Client, &expiresAt, &usedAt, &sessionHash)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if usedAt != nil {
			if sessionHash != nil {
				_, err = tx.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, sessionHash)
			}
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE app_codes SET used_at = now() WHERE code_hash = $1`, codeHash[:]); err != nil {
			return err
		}
		if !expiresAt.After(time.Now()) || !verify(c.Challenge) {
			return nil
		}
		u, err = scanUser(tx.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id::text = $1`, c.UserID))
		if errors.Is(err, ErrNotFound) || (err == nil && u.Disabled) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := createSession(ctx, tx, Session{UserID: u.ID, TokenHash: tokenHash, Expires: expires, SID: deref(sid), Client: c.Client}); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE app_codes SET session_hash = $2 WHERE code_hash = $1`, codeHash[:], tokenHash[:]); err != nil {
			return err
		}
		granted = true
		return nil
	})
	if err != nil {
		return User{}, err
	}
	if !granted {
		return User{}, ErrInvalidGrant
	}
	return u, nil
}

// Receipt kinds.
const (
	ReceiptLogoutToken = "logout_token"
	ReceiptEvent       = "event"
)

// RememberReceipt records a logout token or event ID until expires and
// reports whether it was new; one recorded before that has not expired is
// a replay.
func (s *Store) RememberReceipt(ctx context.Context, kind, id string, expires time.Time) (bool, error) {
	var fresh bool
	err := s.pool.QueryRow(ctx, `INSERT INTO idp_receipts (kind, id, expires_at) VALUES ($1, $2, $3)
		ON CONFLICT (kind, id) DO UPDATE SET expires_at = excluded.expires_at WHERE idp_receipts.expires_at <= now()
		RETURNING true`, kind, id, expires).Scan(&fresh)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return fresh, err
}

// SeenReceipt reports whether the ID is recorded and has not expired.
func (s *Store) SeenReceipt(ctx context.Context, kind, id string) (bool, error) {
	var seen bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM idp_receipts WHERE kind = $1 AND id = $2 AND expires_at > now())`,
		kind, id).Scan(&seen)
	return seen, err
}

// disabledByIdentity is users.disabled_by for an account the identity
// provider disabled; no admin's email looks like it. disabled_reason then
// holds the provider's state: deactivated, locked or removed.
const disabledByIdentity = "idp"

// What ApplyIdentityState did.
const (
	IdentityIgnored        = "ignored"
	IdentityUnchanged      = "unchanged"
	IdentityDeleted        = "deleted"
	IdentityDisabled       = "disabled"
	IdentityRestored       = "restored"
	IdentityPendingRemoval = "pending_removal"
)

// ApplyIdentityState brings the account linked to the identity in line with
// what the provider says of it now (docs/design/accounts.md section 1).
// Removed deletes the account as DeleteUser does, except that an admin with
// no other active admin beside it is disabled instead and left for
// FinishIdentityRemovals, so the setup wizard does not open to anyone.
// Deactivated and locked disable the account and end its sessions; active
// restores it when the provider disabled it, and never undoes an admin's
// disabling. A subject no account is linked to is ignored.
func (s *Store) ApplyIdentityState(ctx context.Context, issuer, subject string, state model.IdentityState) (string, error) {
	outcome := IdentityIgnored
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var userID string
		err := tx.QueryRow(ctx, `SELECT user_id::text FROM user_identities WHERE issuer = $1 AND subject = $2`,
			issuer, subject).Scan(&userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		admin, disabled, err := lockUser(ctx, tx, userID)
		if err != nil {
			return err
		}
		var by, reason string
		if err := tx.QueryRow(ctx, `SELECT disabled_by, disabled_reason FROM users WHERE id::text = $1`, userID).Scan(&by, &reason); err != nil {
			return err
		}
		ours := disabled && by == disabledByIdentity
		outcome = IdentityUnchanged
		switch state {
		case model.IdentityRemoved:
			if admin {
				var others int
				if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE is_admin AND disabled_at IS NULL AND id::text <> $1`,
					userID).Scan(&others); err != nil {
					return err
				}
				if others == 0 {
					if !ours || reason != string(model.IdentityRemoved) {
						outcome = IdentityPendingRemoval
					}
					return disableForIdentity(ctx, tx, userID, model.IdentityRemoved)
				}
			}
			outcome = IdentityDeleted
			_, err := tx.Exec(ctx, `DELETE FROM users WHERE id::text = $1`, userID)
			return err
		case model.IdentityDeactivated, model.IdentityLocked:
			if (disabled && !ours) || (ours && (reason == string(state) || reason == string(model.IdentityRemoved))) {
				return nil
			}
			outcome = IdentityDisabled
			return disableForIdentity(ctx, tx, userID, state)
		case model.IdentityActive:
			if !ours || reason == string(model.IdentityRemoved) {
				return nil
			}
			outcome = IdentityRestored
			_, err := tx.Exec(ctx, `UPDATE users SET disabled_at = NULL, disabled_reason = '', disabled_by = '' WHERE id::text = $1`, userID)
			return err
		}
		return fmt.Errorf("unknown identity state %q", state)
	})
	return outcome, err
}

func disableForIdentity(ctx context.Context, tx pgx.Tx, userID string, state model.IdentityState) error {
	if _, err := tx.Exec(ctx, `UPDATE users SET disabled_at = coalesce(disabled_at, now()), disabled_reason = $2, disabled_by = $3
		WHERE id::text = $1`, userID, string(state), disabledByIdentity); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_id::text = $1`, userID)
	return err
}

// FinishIdentityRemovals deletes the accounts whose identity was removed
// while they were the only active admin, once another active admin exists.
func (s *Store) FinishIdentityRemovals(ctx context.Context) (int64, error) {
	var n int64
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(1517, 1)`); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `DELETE FROM users u WHERE u.disabled_by = $1 AND u.disabled_reason = $2
			AND (NOT u.is_admin OR EXISTS (SELECT 1 FROM users a WHERE a.is_admin AND a.disabled_at IS NULL AND a.id <> u.id))`,
			disabledByIdentity, string(model.IdentityRemoved))
		n = tag.RowsAffected()
		return err
	})
	return n, err
}

// IdentityCursor is how far the reconciliation has read the issuer's
// events; the zero time when it has not started.
func (s *Store) IdentityCursor(ctx context.Context, issuer string) (time.Time, error) {
	var t time.Time
	err := s.pool.QueryRow(ctx, `SELECT read_until FROM idp_cursors WHERE issuer = $1`, issuer).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil
	}
	return t, err
}

func (s *Store) SetIdentityCursor(ctx context.Context, issuer string, t time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO idp_cursors (issuer, read_until) VALUES ($1, $2)
		ON CONFLICT (issuer) DO UPDATE SET read_until = excluded.read_until, updated_at = now()`, issuer, t)
	return err
}

// PurgeSignIns deletes sign-ins that were never finished, app codes past
// use and receipts no longer needed against replays.
func (s *Store) PurgeSignIns(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `WITH flows AS (DELETE FROM login_flows WHERE expires_at < now()),
		codes AS (DELETE FROM app_codes WHERE expires_at < now() - interval '1 day')
		DELETE FROM idp_receipts WHERE expires_at < now()`)
	return err
}
