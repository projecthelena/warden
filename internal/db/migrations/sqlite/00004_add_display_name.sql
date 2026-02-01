-- +goose Up
ALTER TABLE users ADD COLUMN display_name TEXT;

-- +goose Down
-- SQLite doesn't support DROP COLUMN easily
