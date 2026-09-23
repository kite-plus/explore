-- +goose Up
ALTER TABLE entries
    ADD COLUMN link_status text NOT NULL DEFAULT 'unknown'
        CHECK (link_status IN ('unknown', 'available', 'unavailable')),
    ADD COLUMN link_checked_at timestamptz,
    ADD COLUMN link_next_check_at timestamptz NOT NULL DEFAULT now();

CREATE INDEX entries_link_due ON entries (link_next_check_at, id);

-- +goose Down
