-- +goose Up
-- An open outage is the unit the alerting layer reasons about: these two columns are
-- what it remembers about an outage it has already spoken about, so a restart doesn't
-- re-alert everything that is still down.
ALTER TABLE monitor_outages ADD COLUMN IF NOT EXISTS notified_at TIMESTAMP;
ALTER TABLE monitor_outages ADD COLUMN IF NOT EXISTS last_reminder_at TIMESTAMP;

CREATE INDEX IF NOT EXISTS idx_monitor_outages_open ON monitor_outages(end_time, monitor_id);

-- +goose Down
DROP INDEX IF EXISTS idx_monitor_outages_open;
ALTER TABLE monitor_outages DROP COLUMN IF EXISTS last_reminder_at;
ALTER TABLE monitor_outages DROP COLUMN IF EXISTS notified_at;
