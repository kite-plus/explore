-- +goose Up
-- The ID people see, in sign-up order. The uuid stays the internal key every
-- other table and the identity service refer to. See docs/design/accounts.md.
ALTER TABLE users ADD COLUMN number bigint;

UPDATE users u SET number = ranked.n
FROM (SELECT id, row_number() OVER (ORDER BY created_at, id) AS n FROM users) ranked
WHERE ranked.id = u.id;

ALTER TABLE users ALTER COLUMN number SET NOT NULL;
-- ALWAYS: numbers come only from the sequence and are never changed later.
ALTER TABLE users ALTER COLUMN number ADD GENERATED ALWAYS AS IDENTITY;
SELECT setval(pg_get_serial_sequence('users', 'number'), coalesce(max(number), 0) + 1, false) FROM users;
ALTER TABLE users ADD CONSTRAINT users_number_key UNIQUE (number);

-- +goose Down
-- Production only migrates forward; see docs/design/data-model.md section 7.
