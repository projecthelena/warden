-- +goose Up
ALTER TABLE users ADD COLUMN avatar_url TEXT;

-- +goose Down
-- SQLite doesn't support DROP COLUMN easily, so we leave it
