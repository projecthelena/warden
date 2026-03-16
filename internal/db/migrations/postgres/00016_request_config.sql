-- +goose Up
ALTER TABLE monitors ADD COLUMN request_config TEXT DEFAULT NULL;

-- +goose Down
ALTER TABLE monitors DROP COLUMN IF EXISTS request_config;
