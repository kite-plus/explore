-- +goose Up
-- Setup no longer asks for a code: the wizard creates the first admin
-- account directly, so the table that held the code goes.
DROP TABLE setup_code;

-- +goose Down
CREATE TABLE setup_code (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    code text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
