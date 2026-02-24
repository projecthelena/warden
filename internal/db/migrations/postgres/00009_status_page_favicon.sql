-- +goose Up
-- Add favicon_url column to status pages for custom favicon support
ALTER TABLE status_pages ADD COLUMN favicon_url TEXT DEFAULT '';

-- +goose Down
ALTER TABLE status_pages DROP COLUMN favicon_url;
