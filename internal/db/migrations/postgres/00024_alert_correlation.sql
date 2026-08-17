-- +goose Up
-- Outages that were announced together as one incident share a correlation id, so the
-- reminders that follow stay one message instead of one per monitor.
ALTER TABLE monitor_outages ADD COLUMN IF NOT EXISTS correlation_id TEXT;

-- A monitor whose alerts are muted still opens outages and still appears in the daily
-- digest; it just never interrupts anyone. Meant for test and staging targets.
ALTER TABLE monitors ADD COLUMN IF NOT EXISTS alerts_muted BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_monitor_outages_notified ON monitor_outages(monitor_id, notified_at);

-- +goose Down
DROP INDEX IF EXISTS idx_monitor_outages_notified;
ALTER TABLE monitors DROP COLUMN IF EXISTS alerts_muted;
ALTER TABLE monitor_outages DROP COLUMN IF EXISTS correlation_id;
