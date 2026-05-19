-- +goose Up
-- Enriched event details: status code, latency, error message, response body (truncated),
-- and filtered response headers for the check that triggered each event.
ALTER TABLE monitor_events ADD COLUMN status_code INTEGER;
ALTER TABLE monitor_events ADD COLUMN latency INTEGER;
ALTER TABLE monitor_events ADD COLUMN error_message TEXT;
ALTER TABLE monitor_events ADD COLUMN response_body TEXT;
ALTER TABLE monitor_events ADD COLUMN response_headers TEXT;

CREATE INDEX IF NOT EXISTS idx_monitor_events_monitor_id_ts ON monitor_events(monitor_id, timestamp DESC);

INSERT OR IGNORE INTO settings (key, value) VALUES ('app_url', '');

-- +goose Down
DROP INDEX IF EXISTS idx_monitor_events_monitor_id_ts;
ALTER TABLE monitor_events DROP COLUMN response_headers;
ALTER TABLE monitor_events DROP COLUMN response_body;
ALTER TABLE monitor_events DROP COLUMN error_message;
ALTER TABLE monitor_events DROP COLUMN latency;
ALTER TABLE monitor_events DROP COLUMN status_code;
DELETE FROM settings WHERE key = 'app_url';
