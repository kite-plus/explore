-- +goose Up
-- The recommended stream's rating, from the same model call as an entry's
-- tags and cached the same way. See docs/design/accounts.md section 3.1.
-- Entries tagged before it are tagged again, newest first, to be rated.
ALTER TABLE entries
    ADD COLUMN quality smallint CHECK (quality BETWEEN 0 AND 3);

UPDATE entries SET tagged_at = NULL;

-- +goose Down
-- Production only migrates forward; see docs/design/data-model.md section 7.
