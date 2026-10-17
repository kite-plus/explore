-- +goose Up
-- Search over posts and blogs; see docs/design/data-model.md section 4.4.
-- PostgreSQL's parser takes a run of Chinese or Japanese characters as one
-- word and no dictionary in the stock image splits it, so search_text
-- rewrites each such run as its overlapping pairs plus its last character
-- ("独立博客" becomes "独立 立博 博客 客"). A query asks for adjacent pairs,
-- which matches the characters as typed, two-character words included.
-- Other text stays as it was: whole words under the simple configuration.
-- The character class must match cjk in internal/store/search.go.
-- +goose StatementBegin
CREATE FUNCTION search_text(doc text) RETURNS text
    LANGUAGE sql IMMUTABLE PARALLEL SAFE
    RETURN coalesce((
        SELECT string_agg(
                   CASE WHEN m[1] ~ '^[\u3040-\u30ff\u3400-\u4dbf\u4e00-\u9fff\uac00-\ud7af\uf900-\ufaff\U00020000-\U0003134f]'
                   THEN (SELECT string_agg(substr(m[1], i, 2), ' ' ORDER BY i) FROM generate_series(1, char_length(m[1])) AS i)
                   ELSE m[1] END,
                   ' ' ORDER BY n)
        FROM regexp_matches(doc,
            '[\u3040-\u30ff\u3400-\u4dbf\u4e00-\u9fff\uac00-\ud7af\uf900-\ufaff\U00020000-\U0003134f]+|[^\u3040-\u30ff\u3400-\u4dbf\u4e00-\u9fff\uac00-\ud7af\uf900-\ufaff\U00020000-\U0003134f]+',
            'g') WITH ORDINALITY AS r(m, n)
    ), '');
-- +goose StatementEnd

-- Weight A is the title (a blog's name), B the feed's excerpt (a blog's
-- description), so a query can ask for the title alone. Page excerpts stay
-- out: a site's default description repeats on every post of the blog.
ALTER TABLE entries ADD COLUMN search tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('simple', search_text(title)), 'A') ||
    setweight(to_tsvector('simple', search_text(coalesce(excerpt, ''))), 'B')) STORED;
CREATE INDEX entries_search ON entries USING gin (search);

-- A few thousand blogs need no index.
ALTER TABLE blogs ADD COLUMN search tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('simple', search_text(name)), 'A') ||
    setweight(to_tsvector('simple', search_text(description)), 'B')) STORED;

-- +goose Down
-- Production only migrates forward; see docs/design/data-model.md section 7.
