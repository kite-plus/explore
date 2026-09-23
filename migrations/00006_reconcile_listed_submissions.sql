-- +goose Up
UPDATE submissions AS s
SET status = 'approved', blog_id = b.id,
    reviewed_by = coalesce(nullif(s.reviewed_by, ''), 'system'),
    reviewed_at = coalesce(s.reviewed_at, now())
FROM blogs AS b
WHERE s.status = 'pending' AND s.host = b.host;

-- +goose Down
-- Production only migrates forward; see docs/design/data-model.md section 7.
