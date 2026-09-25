-- +goose Up
-- What an entry's own page offers where its feed has no image or cuts the
-- excerpt short: og:image and og:description from the page's head, read
-- once per URL. Like the rest of entries, a cache the pages can rebuild.
-- page_next_check_at is when the head is due; NULL once it has been read.
ALTER TABLE entries
    ADD COLUMN page_image_url     text,
    ADD COLUMN page_excerpt       text CHECK (char_length(page_excerpt) <= 140),
    ADD COLUMN page_next_check_at timestamptz DEFAULT now();

CREATE INDEX entries_page_due ON entries (page_next_check_at) WHERE page_next_check_at IS NOT NULL;

-- +goose Down
DROP INDEX entries_page_due;
ALTER TABLE entries
    DROP COLUMN page_image_url,
    DROP COLUMN page_excerpt,
    DROP COLUMN page_next_check_at;
