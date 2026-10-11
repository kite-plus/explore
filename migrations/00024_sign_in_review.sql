-- +goose Up
-- Follow-ups to sign-in with Kite Plus; see docs/design/accounts.md
-- sections 1 and 1.1.

-- The callback looks for a browser's other pending sign-ins by its binding.
CREATE INDEX login_flows_browser_hash ON login_flows (browser_hash);

-- The email of the linked Kite Plus account, shown on the account page; it
-- may differ from the account's own email, and is refreshed at sign-in.
ALTER TABLE user_identities ADD COLUMN email text NOT NULL DEFAULT '' CHECK (char_length(email) <= 254);

-- An app sign-in that comes back to a custom scheme, waiting for the reader
-- to confirm it in the browser that started it before any code is issued.
CREATE TABLE app_confirmations (
    token_hash bytea PRIMARY KEY,
    browser_hash bytea NOT NULL,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    sid text,
    challenge text NOT NULL,
    client text NOT NULL CHECK (client IN ('ios', 'android')),
    expires_at timestamptz NOT NULL
);
CREATE INDEX app_confirmations_expires_at ON app_confirmations (expires_at);
CREATE INDEX app_confirmations_browser_hash ON app_confirmations (browser_hash);

-- +goose Down
-- Production only migrates forward; see docs/design/data-model.md section 7.
