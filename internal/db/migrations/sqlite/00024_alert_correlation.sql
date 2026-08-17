-- +goose Up
-- Outages that were announced together as one incident share a correlation id, so the
-- reminders that follow stay one message instead of one per monitor.
ALTER TABLE monitor_outages ADD COLUMN correlation_id TEXT;

-- A monitor whose alerts are muted still opens outages and still appears in the daily
-- digest; it just never interrupts anyone. Meant for test and staging targets.
ALTER TABLE monitors ADD COLUMN alerts_muted BOOLEAN NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_monitor_outages_notified ON monitor_outages(monitor_id, notified_at);

-- +goose Down
DROP INDEX IF EXISTS idx_monitor_outages_notified;
ALTER TABLE monitors DROP COLUMN alerts_muted;
ALTER TABLE monitor_outages DROP COLUMN correlation_id;
