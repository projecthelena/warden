-- +goose Up
-- Per-monitor overrides (NULL = use global default)
ALTER TABLE monitors ADD COLUMN confirmation_threshold INTEGER DEFAULT NULL;
ALTER TABLE monitors ADD COLUMN notification_cooldown_minutes INTEGER DEFAULT NULL;

-- Global defaults in settings table
INSERT OR IGNORE INTO settings (key, value) VALUES ('notification.confirmation_threshold', '3');
INSERT OR IGNORE INTO settings (key, value) VALUES ('notification.cooldown_minutes', '30');
INSERT OR IGNORE INTO settings (key, value) VALUES ('notification.flap_detection_enabled', 'true');
INSERT OR IGNORE INTO settings (key, value) VALUES ('notification.flap_window_checks', '21');
INSERT OR IGNORE INTO settings (key, value) VALUES ('notification.flap_threshold_percent', '25');

-- +goose Down
-- SQLite does not support DROP COLUMN before 3.35.0, but goose needs a Down block.
-- For safety, we leave the columns in place on rollback.
DELETE FROM settings WHERE key IN (
    'notification.confirmation_threshold',
    'notification.cooldown_minutes',
    'notification.flap_detection_enabled',
    'notification.flap_window_checks',
    'notification.flap_threshold_percent'
);
