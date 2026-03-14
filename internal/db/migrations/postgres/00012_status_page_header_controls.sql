-- +goose Up
ALTER TABLE status_pages ADD COLUMN header_content TEXT DEFAULT 'logo-title';
ALTER TABLE status_pages ADD COLUMN header_alignment TEXT DEFAULT 'center';
ALTER TABLE status_pages ADD COLUMN header_arrangement TEXT DEFAULT 'stacked';

-- +goose Down
ALTER TABLE status_pages DROP COLUMN IF EXISTS header_content;
ALTER TABLE status_pages DROP COLUMN IF EXISTS header_alignment;
ALTER TABLE status_pages DROP COLUMN IF EXISTS header_arrangement;
