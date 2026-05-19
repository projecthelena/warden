-- +goose Up
ALTER TABLE monitor_events ADD COLUMN IF NOT EXISTS status_code INTEGER;
ALTER TABLE monitor_events ADD COLUMN IF NOT EXISTS latency BIGINT;
ALTER TABLE monitor_events ADD COLUMN IF NOT EXISTS error_message TEXT;
ALTER TABLE monitor_events ADD COLUMN IF NOT EXISTS response_body TEXT;
ALTER TABLE monitor_events ADD COLUMN IF NOT EXISTS response_headers TEXT;

CREATE INDEX IF NOT EXISTS idx_monitor_events_monitor_id_ts ON monitor_events(monitor_id, timestamp DESC);

INSERT INTO settings (key, value) VALUES ('app_url', '') ON CONFLICT (key) DO NOTHING;

-- +goose Down
DROP INDEX IF EXISTS idx_monitor_events_monitor_id_ts;
ALTER TABLE monitor_events DROP COLUMN IF EXISTS response_headers;
ALTER TABLE monitor_events DROP COLUMN IF EXISTS response_body;
ALTER TABLE monitor_events DROP COLUMN IF EXISTS error_message;
ALTER TABLE monitor_events DROP COLUMN IF EXISTS latency;
ALTER TABLE monitor_events DROP COLUMN IF EXISTS status_code;
DELETE FROM settings WHERE key = 'app_url';
