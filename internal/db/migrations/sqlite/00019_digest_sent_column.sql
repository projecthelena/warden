-- +goose Up
ALTER TABLE notification_digest_queue ADD COLUMN sent INTEGER NOT NULL DEFAULT 0;
ALTER TABLE notification_digest_queue ADD COLUMN sent_at DATETIME;

-- +goose Down
-- SQLite does not support DROP COLUMN in versions prior to 3.35; column additions are left in place on rollback.
