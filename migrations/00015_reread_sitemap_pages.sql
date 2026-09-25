-- +goose Up
-- Pages first read from sitemaps kept a site name in curly quotes in their
-- titles and missed dates in long heads. Reading them again with the fixed
-- worker updates their entries.
UPDATE sitemap_urls SET next_check_at = now() WHERE next_check_at IS NULL;

-- +goose Down
-- Nothing to undo: the pages are only read again.
