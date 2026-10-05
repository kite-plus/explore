-- +goose Up
-- The recommended stream's score, the sum of three marks from the same model
-- call, replaces the four-level quality. See docs/design/accounts.md section
-- 3.1. Entries are rated again, newest first, and keep their tags meanwhile.
ALTER TABLE entries DROP COLUMN quality;
ALTER TABLE entries ADD COLUMN score smallint CHECK (score BETWEEN 0 AND 15);

UPDATE entries SET tagged_at = NULL;

-- +goose Down
-- Production only migrates forward; see docs/design/data-model.md section 7.
