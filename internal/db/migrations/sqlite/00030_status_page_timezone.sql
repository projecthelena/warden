-- +goose Up
ALTER TABLE status_pages ADD COLUMN timezone TEXT NOT NULL DEFAULT 'UTC';

-- +goose Down
-- SQLite does not support dropping this column safely on older versions.
