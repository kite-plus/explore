-- +goose Up
-- Sign-in with the Kite Plus identity provider, for the site and the apps.
-- See docs/design/accounts.md sections 1 and 1.1.

-- The provider's session a sign-in came from, which back-channel logout
-- names, and which client holds the session.
ALTER TABLE sessions
    ADD COLUMN sid text,
    ADD COLUMN client text NOT NULL DEFAULT 'web' CHECK (client IN ('web', 'ios', 'android'));
CREATE INDEX sessions_sid ON sessions (sid) WHERE sid IS NOT NULL;

-- A sign-in waiting for the provider to send the reader back. The state and
-- the cookie that binds it to the starting browser are kept as hashes.
CREATE TABLE login_flows (
    state_hash bytea PRIMARY KEY,
    browser_hash bytea NOT NULL,
    mode text NOT NULL CHECK (mode IN ('web', 'link', 'app')),
    nonce text NOT NULL,
    verifier text NOT NULL,
    return_path text NOT NULL DEFAULT '',
    user_id uuid REFERENCES users (id) ON DELETE CASCADE,
    challenge text,
    client text NOT NULL DEFAULT 'web' CHECK (client IN ('web', 'ios', 'android')),
    app_redirect text NOT NULL DEFAULT '',
    expires_at timestamptz NOT NULL,
    CHECK ((mode = 'link') = (user_id IS NOT NULL)),
    CHECK ((mode = 'app') = (challenge IS NOT NULL))
);
CREATE INDEX login_flows_expires_at ON login_flows (expires_at);

-- A one-time code an app exchanges for a session. session_hash is the
-- session it was exchanged for, ended if the code is tried again.
CREATE TABLE app_codes (
    code_hash bytea PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    sid text,
    challenge text NOT NULL,
    client text NOT NULL CHECK (client IN ('ios', 'android')),
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    session_hash bytea
);
CREATE INDEX app_codes_expires_at ON app_codes (expires_at);

-- Logout token IDs and pushed event IDs seen recently, to refuse replays.
CREATE TABLE idp_receipts (
    kind text NOT NULL CHECK (kind IN ('logout_token', 'event')),
    id text NOT NULL,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (kind, id)
);

-- How far the reconciliation has read the provider's user events.
CREATE TABLE idp_cursors (
    issuer text PRIMARY KEY,
    read_until timestamptz NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
-- Production only migrates forward; see docs/design/data-model.md section 7.
