-- +goose Up
-- Instability remains detectable, but immediate notices become opt-in on upgrade.
INSERT INTO settings (key, value) VALUES ('notification.event.flapping.enabled', 'false')
ON CONFLICT (key) DO UPDATE SET value = excluded.value;

ALTER TABLE monitor_outages ADD COLUMN notification_continuation BOOLEAN NOT NULL DEFAULT FALSE;
CREATE TABLE notification_cooldowns (
    monitor_id TEXT NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    notified_at TIMESTAMP NOT NULL,
    PRIMARY KEY (monitor_id, event_type)
);

-- Replace the former reminder defaults; preserve explicitly customized schedules.
UPDATE settings SET value = '0'
WHERE key IN ('notification.alert.reminder_minutes', 'notification.alert.repeat_reminder_minutes')
AND NOT EXISTS (
    SELECT 1 FROM settings existing
    WHERE (existing.key = 'notification.alert.reminder_minutes' AND existing.value <> '30')
       OR (existing.key = 'notification.alert.repeat_reminder_minutes' AND existing.value <> '60')
);

CREATE TABLE notification_recoveries (
    monitor_id TEXT PRIMARY KEY REFERENCES monitors(id) ON DELETE CASCADE,
    recovered_at TIMESTAMP NOT NULL,
    notified_at TIMESTAMP NOT NULL,
    outage_type TEXT NOT NULL,
    correlation_id TEXT NOT NULL,
    payload TEXT NOT NULL
);

-- +goose Down
ALTER TABLE monitor_outages DROP COLUMN notification_continuation;
DROP TABLE notification_recoveries;
DROP TABLE notification_cooldowns;
