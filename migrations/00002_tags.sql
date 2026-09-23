-- +goose Up
-- Tags are cache like the rest of an entry: the worker files each post
-- under at most three tags from the list in internal/model, from its title,
-- excerpt and the categories its feed gives. See docs/design/accounts.md
-- section 5.
ALTER TABLE entries
    ADD COLUMN categories text[]      NOT NULL DEFAULT '{}' CHECK (cardinality(categories) <= 10),
    ADD COLUMN tags       text[]      NOT NULL DEFAULT '{}' CHECK (cardinality(tags) <= 3),
    ADD COLUMN tagged_at  timestamptz;

CREATE INDEX entries_tags     ON entries USING gin (tags);
CREATE INDEX entries_untagged ON entries (published_at DESC NULLS LAST, id DESC) WHERE tagged_at IS NULL;

-- A maintainer's hint for the tagger, part of the blog list.
ALTER TABLE blogs
    ADD COLUMN default_tags text[] NOT NULL DEFAULT '{}' CHECK (cardinality(default_tags) <= 3);

-- +goose Down
-- Production only migrates forward; see docs/design/data-model.md section 7.
