package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ErrSetupDone is returned once an admin account exists.
var ErrSetupDone = errors.New("setup is already complete")

// SetupPending reports whether the server is still waiting for its first
// admin account.
func (s *Store) SetupPending(ctx context.Context) (bool, error) {
	var pending bool
	err := s.pool.QueryRow(ctx, `SELECT NOT EXISTS (SELECT 1 FROM users WHERE is_admin)`).Scan(&pending)
	return pending, err
}

// CompleteSetup creates the first admin account and applies the chosen
// settings, all or nothing. Once an admin exists it returns ErrSetupDone.
func (s *Store) CompleteSetup(ctx context.Context, email, passwordHash, displayName string, settings map[string]string) (User, error) {
	var u User
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// The lock admin changes take, so two setups cannot both succeed.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(1517, 1)`); err != nil {
			return err
		}
		var done bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE is_admin)`).Scan(&done); err != nil {
			return err
		}
		if done {
			return ErrSetupDone
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
		return nil
	})
	return u, err
}
