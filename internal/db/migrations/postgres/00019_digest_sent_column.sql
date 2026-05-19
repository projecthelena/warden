-- +goose Up
ALTER TABLE notification_digest_queue ADD COLUMN sent BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE notification_digest_queue ADD COLUMN sent_at TIMESTAMP;

-- +goose Down
ALTER TABLE notification_digest_queue DROP COLUMN IF EXISTS sent_at;
ALTER TABLE notification_digest_queue DROP COLUMN IF EXISTS sent;
