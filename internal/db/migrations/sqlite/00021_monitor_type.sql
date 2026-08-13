-- +goose Up
ALTER TABLE monitors ADD COLUMN type TEXT NOT NULL DEFAULT 'http';

-- +goose Down
-- SQLite does not support DROP COLUMN before 3.35.0
