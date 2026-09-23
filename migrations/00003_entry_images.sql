-- +goose Up
-- Only the source URL is cached. Image bytes are never stored in PostgreSQL.
ALTER TABLE entries ADD COLUMN image_url text CHECK (char_length(image_url) <= 2000);

-- +goose Down
-- Production only migrates forward; see docs/design/data-model.md section 7.
