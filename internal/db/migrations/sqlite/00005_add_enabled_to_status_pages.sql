-- +goose Up
ALTER TABLE status_pages ADD COLUMN enabled BOOLEAN DEFAULT FALSE;
UPDATE status_pages SET enabled = public;

-- +goose Down
-- SQLite does not support DROP COLUMN before 3.35.0, but goose needs a Down block.
-- For safety, we leave the column in place on rollback.
