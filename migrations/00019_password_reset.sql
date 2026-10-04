-- +goose Up
-- Set when an admin resets an account's password and cleared when its owner
-- changes it, so both sides can tell a temporary password is still in use.
-- See docs/design/admin-operations.md.
ALTER TABLE users
    ADD COLUMN password_reset_at timestamptz,
    ADD COLUMN password_reset_by text NOT NULL DEFAULT '';

-- +goose Down
-- Production only migrates forward; see docs/design/data-model.md section 7.
