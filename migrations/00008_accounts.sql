-- +goose Up
CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL UNIQUE CHECK (char_length(email) <= 254),
    password_hash text NOT NULL,
    display_name text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 80),
    is_admin boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    token_hash bytea PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);
CREATE INDEX sessions_user_id ON sessions(user_id);
CREATE INDEX sessions_expires_at ON sessions(expires_at);

CREATE TABLE user_identities (
    issuer text NOT NULL,
    subject text NOT NULL,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    linked_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (issuer, subject),
    UNIQUE (user_id, issuer)
);

CREATE TABLE subscriptions (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blog_id bigint NOT NULL REFERENCES blogs(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, blog_id)
);
CREATE INDEX subscriptions_blog_id ON subscriptions(blog_id);

CREATE TABLE blog_owners (
    blog_id bigint PRIMARY KEY REFERENCES blogs(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    verified_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX blog_owners_user_id ON blog_owners(user_id);

CREATE TABLE blog_claim_challenges (
    blog_id bigint NOT NULL REFERENCES blogs(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (blog_id, user_id)
);

-- +goose Down
DROP TABLE blog_claim_challenges;
DROP TABLE blog_owners;
DROP TABLE subscriptions;
DROP TABLE user_identities;
DROP TABLE sessions;
DROP TABLE users;
