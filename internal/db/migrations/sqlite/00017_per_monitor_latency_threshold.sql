-- +goose Up
ALTER TABLE monitors ADD COLUMN latency_threshold INTEGER DEFAULT NULL;

-- +goose Down
-- SQLite does not support DROP COLUMN before 3.35.0
