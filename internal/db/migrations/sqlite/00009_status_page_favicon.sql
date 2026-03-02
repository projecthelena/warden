-- +goose Up
-- Add favicon_url column to status pages for custom favicon support
ALTER TABLE status_pages ADD COLUMN favicon_url TEXT DEFAULT '';

-- +goose Down
-- SQLite doesn't support DROP COLUMN, so we leave the column in place for down migration
