-- +goose Up
CREATE TABLE blogs (
    id                   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    host                 text        NOT NULL UNIQUE,
    name                 text        NOT NULL CHECK (char_length(name) <= 100),
    site_url             text        NOT NULL,
    feed_url             text        NOT NULL,
    language             text        NOT NULL,
    generator            text        NOT NULL DEFAULT 'unknown',
    status               text        NOT NULL DEFAULT 'active'
                                     CHECK (status IN ('active', 'paused')),
    status_note          text,
    show_excerpt         boolean     NOT NULL DEFAULT true,
    extra_domains        text[]      NOT NULL DEFAULT '{}',

    etag                 text,
    last_modified        text,
    body_hash            bytea,
    fetch_interval       interval    NOT NULL DEFAULT '60 minutes',
    next_fetch_at        timestamptz NOT NULL DEFAULT now(),
    last_fetched_at      timestamptz,
    last_succeeded_at    timestamptz,
    consecutive_failures integer     NOT NULL DEFAULT 0,
    last_error           text,
    gone_since           timestamptz,

    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX blogs_due ON blogs (next_fetch_at) WHERE status = 'active';

-- A cache of each blog's current feed. There is deliberately no column for
-- post content: see docs/design/architecture.md section 0.1.
CREATE TABLE entries (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    blog_id       bigint      NOT NULL REFERENCES blogs (id) ON DELETE CASCADE,
    identity      text        NOT NULL CHECK (char_length(identity) <= 500),
    url           text        NOT NULL CHECK (char_length(url) <= 2000),
    title         text        NOT NULL CHECK (char_length(title) <= 300),
    excerpt       text                 CHECK (char_length(excerpt) <= 140),
    published_at  timestamptz,
    date_trusted  boolean     NOT NULL,
    synced_at     timestamptz NOT NULL DEFAULT now(),

    UNIQUE (blog_id, identity),
    CHECK (published_at IS NOT NULL OR NOT date_trusted)
);

CREATE INDEX entries_stream  ON entries (published_at DESC, id DESC) WHERE date_trusted;
CREATE INDEX entries_by_blog ON entries (blog_id, published_at DESC NULLS LAST);

CREATE TABLE submissions (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    host          text        NOT NULL,
    site_url      text        NOT NULL,
    feed_url      text        NOT NULL,
    note          text        CHECK (char_length(note) <= 500),
    check_report  jsonb       NOT NULL,
    status        text        NOT NULL DEFAULT 'pending'
                              CHECK (status IN ('pending', 'approved', 'rejected')),
    review_note   text,
    reviewed_by   text,
    reviewed_at   timestamptz,
    blog_id       bigint      REFERENCES blogs (id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX submissions_one_pending_per_host ON submissions (host) WHERE status = 'pending';

CREATE TABLE excluded_hosts (
    host        text        PRIMARY KEY,
    reason      text        NOT NULL CHECK (reason IN ('opt_out', 'blocked')),
    note        text,
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
-- Production only migrates forward; see docs/design/data-model.md section 7.
