-- +goose Up
ALTER TABLE monitors ADD COLUMN request_config TEXT DEFAULT NULL;

-- +goose Down
-- SQLite does not support DROP COLUMN before 3.35.0
