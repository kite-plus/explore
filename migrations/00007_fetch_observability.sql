-- +goose Up
CREATE TABLE fetch_attempts (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    blog_id     bigint NOT NULL REFERENCES blogs (id) ON DELETE CASCADE,
    started_at  timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    outcome     text NOT NULL DEFAULT 'running'
                     CHECK (outcome IN ('running', 'changed', 'unchanged', 'failed')),
    http_status integer,
    entry_count integer,
    error       text
);

CREATE INDEX fetch_attempts_by_blog ON fetch_attempts (blog_id, id DESC);

CREATE TABLE worker_heartbeats (
    worker_id    text PRIMARY KEY,
    started_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
-- Production only migrates forward; see docs/design/data-model.md section 7.
