-- +goose Up
ALTER TABLE blogs
    ADD COLUMN description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 240),
    ADD COLUMN description_checked_at timestamptz;

-- +goose Down
-- Production only migrates forward; see docs/design/data-model.md section 7.
