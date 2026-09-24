-- +goose Up
-- Until the first admin account exists, finishing setup takes this code,
-- which serve prints to its log: only someone who can read the server's logs
-- can claim a fresh install. The single row goes when setup is done.
CREATE TABLE setup_code (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    code text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE setup_code;
