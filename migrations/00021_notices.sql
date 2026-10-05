-- +goose Up
-- Notices and ads Explore publishes itself, shown in the latest stream. See
-- docs/design/notices.md; data-model.md section 2.5 has the columns.
CREATE TABLE notices (
    id          bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind        text        NOT NULL CHECK (kind IN ('notice', 'ad')),
    title       text        NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
    summary     text        NOT NULL DEFAULT '' CHECK (char_length(summary) <= 280),
    body        text        NOT NULL DEFAULT '' CHECK (char_length(body) <= 20000),
    url         text        NOT NULL DEFAULT '' CHECK (url = '' OR url ~ '^https?://'),
    source_name text        NOT NULL CHECK (char_length(source_name) BETWEEN 1 AND 60),
    position    smallint    NOT NULL DEFAULT 0 CHECK (position BETWEEN 0 AND 50),
    audience    text        NOT NULL DEFAULT '' CHECK (audience IN ('', 'zh', 'en')),
    starts_at   timestamptz,
    ends_at     timestamptz CHECK (ends_at IS NULL OR starts_at IS NULL OR ends_at > starts_at),
    enabled     boolean     NOT NULL DEFAULT false,
    updated_by  text        NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CHECK (url <> '' OR body <> '')
);

-- +goose Down
-- Production only migrates forward; see docs/design/data-model.md section 7.
