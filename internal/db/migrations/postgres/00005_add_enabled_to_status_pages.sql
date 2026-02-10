-- +goose Up
ALTER TABLE status_pages ADD COLUMN enabled BOOLEAN DEFAULT FALSE;
UPDATE status_pages SET enabled = public;

-- +goose Down
ALTER TABLE status_pages DROP COLUMN enabled;
