-- +goose Up
ALTER TABLE monitors ADD COLUMN latency_threshold INTEGER DEFAULT NULL;

-- +goose Down
ALTER TABLE monitors DROP COLUMN IF EXISTS latency_threshold;
