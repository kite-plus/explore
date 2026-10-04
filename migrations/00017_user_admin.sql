-- +goose Up
-- Why and by whom an account was disabled, and when it was last used. See
-- docs/design/admin-operations.md.
ALTER TABLE users
    ADD COLUMN disabled_reason text NOT NULL DEFAULT '' CHECK (char_length(disabled_reason) <= 500),
    ADD COLUMN disabled_by text NOT NULL DEFAULT '',
    ADD COLUMN last_seen_at timestamptz;

-- The newest live session is the best guess for accounts used before this.
UPDATE users u SET last_seen_at = (SELECT max(s.created_at) FROM sessions s WHERE s.user_id = u.id);

-- +goose Down
-- Production only migrates forward; see docs/design/data-model.md section 7.
