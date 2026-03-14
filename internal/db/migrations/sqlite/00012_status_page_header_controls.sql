-- +goose Up
ALTER TABLE status_pages ADD COLUMN header_content TEXT DEFAULT 'logo-title';
ALTER TABLE status_pages ADD COLUMN header_alignment TEXT DEFAULT 'center';
ALTER TABLE status_pages ADD COLUMN header_arrangement TEXT DEFAULT 'stacked';

-- +goose Down
-- SQLite doesn't support DROP COLUMN
