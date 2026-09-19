-- +goose Up
ALTER TABLE status_pages ADD COLUMN timezone TEXT NOT NULL DEFAULT 'UTC';

-- +goose Down
ALTER TABLE status_pages DROP COLUMN IF EXISTS timezone;
