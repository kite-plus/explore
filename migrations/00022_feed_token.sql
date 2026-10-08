-- +goose Up
-- The token of the private address that publishes a reader's following
-- stream, for a blogroll on their own site; NULL while they keep it off.
-- See docs/design/accounts.md section 2.
ALTER TABLE users ADD COLUMN feed_token text UNIQUE;

-- +goose Down
-- Production only migrates forward; see docs/design/data-model.md section 7.
