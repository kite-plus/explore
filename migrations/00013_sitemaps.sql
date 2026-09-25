-- +goose Up
-- A blog's sitemap brings in the posts its feed no longer carries. The post
-- addresses it lists and the entries read from their pages are a cache like
-- the feed's: the sitemap and the pages rebuild them.

-- A post's two addresses, one from the feed and one from the sitemap, agree
-- on this key: no scheme, no "www.", no fragment, no trailing slash, lower
-- case. Entries meet on it, and so does a hidden post found again.
-- +goose StatementBegin
CREATE FUNCTION entry_url_key(url text) RETURNS text
    LANGUAGE sql IMMUTABLE PARALLEL SAFE
    RETURN regexp_replace(regexp_replace(lower(split_part(url, '#', 1)), '^[a-z][a-z0-9+.-]*://(www\.)?', ''), '/+(\?|$)', '\1');
-- +goose StatementEnd

ALTER TABLE entries
    ADD COLUMN source  text NOT NULL DEFAULT 'feed' CHECK (source IN ('feed', 'sitemap')),
    ADD COLUMN url_key text GENERATED ALWAYS AS (entry_url_key(url)) STORED;
CREATE INDEX entries_url_key ON entries (blog_id, url_key);

ALTER TABLE suppressed_entries ADD COLUMN url_key text;
UPDATE suppressed_entries se SET url_key = e.url_key
FROM entries e WHERE e.blog_id = se.blog_id AND e.identity = se.identity;

ALTER TABLE blogs ADD COLUMN sitemap_next_check_at timestamptz NOT NULL DEFAULT now();

-- next_check_at is when the page is due to be read: NULL once it has been,
-- until the sitemap gives the address a new lastmod.
CREATE TABLE sitemap_urls (
    blog_id       bigint      NOT NULL REFERENCES blogs (id) ON DELETE CASCADE,
    url           text        NOT NULL CHECK (char_length(url) <= 2000),
    url_key       text        GENERATED ALWAYS AS (entry_url_key(url)) STORED,
    lastmod       timestamptz,
    next_check_at timestamptz DEFAULT now(),
    PRIMARY KEY (blog_id, url)
);
CREATE INDEX sitemap_urls_due ON sitemap_urls (next_check_at) WHERE next_check_at IS NOT NULL;

-- +goose Down
DROP TABLE sitemap_urls;
ALTER TABLE blogs DROP COLUMN sitemap_next_check_at;
ALTER TABLE suppressed_entries DROP COLUMN url_key;
DROP INDEX entries_url_key;
ALTER TABLE entries DROP COLUMN url_key, DROP COLUMN source;
DROP FUNCTION entry_url_key(text);
