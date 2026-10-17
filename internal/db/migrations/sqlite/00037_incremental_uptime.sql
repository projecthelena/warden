-- +goose Up
ALTER TABLE monitor_checks ADD COLUMN ingest_id TEXT;
CREATE UNIQUE INDEX idx_monitor_checks_ingest_id ON monitor_checks(ingest_id) WHERE ingest_id IS NOT NULL;

-- +goose Down
DROP INDEX idx_monitor_checks_ingest_id;
ALTER TABLE monitor_checks DROP COLUMN ingest_id;

DELETE FROM settings WHERE key = 'uptime.incremental.initialized';
