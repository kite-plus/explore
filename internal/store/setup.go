package store

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// setupAlphabet leaves out letters and digits that read alike.
const setupAlphabet = "ABCDEFGHJKMNPQRSTVWXYZ23456789"

var (
	// ErrSetupDone is returned once an admin account exists.
	ErrSetupDone = errors.New("setup is already complete")
	// ErrSetupCode is returned for a code that is not the setup code.
	ErrSetupCode = errors.New("wrong setup code")
)

// SetupPending reports whether the server is still waiting for its first
// admin account.
func (s *Store) SetupPending(ctx context.Context) (bool, error) {
	var pending bool
	err := s.pool.QueryRow(ctx, `SELECT NOT EXISTS (SELECT 1 FROM users WHERE is_admin)`).Scan(&pending)
	return pending, err
}

// SetupCode returns the code that completes setup, creating it on first use.
// It stays the same across restarts until setup is done.
func (s *Store) SetupCode(ctx context.Context) (string, error) {
	fresh, err := newSetupCode()
	if err != nil {
		return "", err
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO setup_code (code) VALUES ($1) ON CONFLICT DO NOTHING`, fresh); err != nil {
		return "", err
	}
	var code string
	err = s.pool.QueryRow(ctx, `SELECT code FROM setup_code`).Scan(&code)
	return code, err
}

// CheckSetupCode reports ErrSetupDone, ErrSetupCode or nil for a code.
func (s *Store) CheckSetupCode(ctx context.Context, code string) error {
	return checkSetupCode(ctx, s.pool, code)
}

// CompleteSetup creates the first admin account, applies the chosen
// settings and forgets the code, all or nothing.
func (s *Store) CompleteSetup(ctx context.Context, code, email, passwordHash, displayName string, settings map[string]string) (User, error) {
	var u User
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// The lock admin changes take, so two setups cannot both succeed.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(1517, 1)`); err != nil {
			return err
		}
		if err := checkSetupCode(ctx, tx, code); err != nil {
			return err
		}
		var err error
		u, err = scanUser(tx.QueryRow(ctx, `INSERT INTO users (email, password_hash, display_name, is_admin)
			VALUES ($1, $2, $3, true) RETURNING id::text, email, password_hash, display_name, is_admin, disabled_at IS NOT NULL`,
			email, passwordHash, displayName))
		if isUniqueViolation(err) {
			return ErrConflict
		}
		if err != nil {
			return err
		}
		for key, value := range settings {
			if _, err := tx.Exec(ctx, `UPDATE system_settings SET value = $2, updated_by = $3, updated_at = now() WHERE key = $1`,
				key, value, email); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `DELETE FROM setup_code`)
		return err
	})
	return u, err
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func checkSetupCode(ctx context.Context, q querier, code string) error {
	var done bool
	if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE is_admin)`).Scan(&done); err != nil {
		return err
	}
	if done {
		return ErrSetupDone
	}
	var stored string
	err := q.QueryRow(ctx, `SELECT code FROM setup_code`).Scan(&stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSetupCode
	}
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(plainCode(stored)), []byte(plainCode(code))) != 1 {
		return ErrSetupCode
	}
	return nil
}

// plainCode drops case, spaces and dashes, which people type either way.
func plainCode(code string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(code) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// newSetupCode makes XXXX-XXXX-XXXX from setupAlphabet, about 59 bits.
func newSetupCode() (string, error) {
	var out []byte
	buf := make([]byte, 32)
	for len(out) < 12 {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for _, b := range buf {
			// Rejecting the top of the byte range keeps every letter
			// equally likely.
			if int(b) < 256-256%len(setupAlphabet) && len(out) < 12 {
				out = append(out, setupAlphabet[int(b)%len(setupAlphabet)])
			}
		}
	}
	return string(out[:4]) + "-" + string(out[4:8]) + "-" + string(out[8:]), nil
}
