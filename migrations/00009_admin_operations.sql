-- +goose Up
ALTER TABLE users ADD COLUMN disabled_at timestamptz;

CREATE TABLE system_settings (
    key text PRIMARY KEY,
    value text NOT NULL,
    updated_by text NOT NULL DEFAULT 'system',
    updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO system_settings (key, value) VALUES
    ('registration_enabled', 'true'),
    ('submissions_enabled', 'true'),
    ('crawler_paused', 'false'),
    ('site_notice', '');

CREATE TABLE suppressed_entries (
    blog_id bigint NOT NULL REFERENCES blogs(id) ON DELETE CASCADE,
    identity text NOT NULL,
    reason text NOT NULL CHECK (char_length(reason) BETWEEN 1 AND 500),
    reviewed_by text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (blog_id, identity)
);

CREATE TABLE takedown_requests (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    target_type text NOT NULL CHECK (target_type IN ('blog', 'entry')),
    blog_id bigint NOT NULL REFERENCES blogs(id) ON DELETE CASCADE,
    entry_identity text,
    requester_id uuid REFERENCES users(id) ON DELETE SET NULL,
    reason text NOT NULL CHECK (char_length(reason) BETWEEN 5 AND 1000),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    review_note text,
    reviewed_by text,
    reviewed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((target_type = 'entry') = (entry_identity IS NOT NULL))
);
CREATE INDEX takedown_requests_status_created ON takedown_requests(status, created_at DESC);
CREATE UNIQUE INDEX takedown_one_pending ON takedown_requests(blog_id, target_type, coalesce(entry_identity, '')) WHERE status = 'pending';

-- +goose Down
DROP TABLE takedown_requests;
DROP TABLE suppressed_entries;
DROP TABLE system_settings;
ALTER TABLE users DROP COLUMN disabled_at;
