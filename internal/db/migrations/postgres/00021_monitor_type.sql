-- +goose Up
ALTER TABLE monitors ADD COLUMN type TEXT NOT NULL DEFAULT 'http';

-- +goose Down
ALTER TABLE monitors DROP COLUMN IF EXISTS type;
