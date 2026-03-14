-- +goose Up
ALTER TABLE status_pages ADD COLUMN header_layout TEXT DEFAULT 'centered';

-- +goose Down
ALTER TABLE status_pages DROP COLUMN IF EXISTS header_layout;
