-- +goose Up
ALTER TABLE status_pages ADD COLUMN header_layout TEXT DEFAULT 'centered';

-- +goose Down
-- SQLite doesn't support DROP COLUMN, so we leave the column in place for down migration
