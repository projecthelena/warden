-- +goose Up
-- Per-monitor overrides (NULL = use global default)
ALTER TABLE monitors ADD COLUMN confirmation_threshold INTEGER DEFAULT NULL;
ALTER TABLE monitors ADD COLUMN notification_cooldown_minutes INTEGER DEFAULT NULL;

-- Global defaults in settings table
INSERT INTO settings (key, value) VALUES ('notification.confirmation_threshold', '3') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value) VALUES ('notification.cooldown_minutes', '30') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value) VALUES ('notification.flap_detection_enabled', 'true') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value) VALUES ('notification.flap_window_checks', '21') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value) VALUES ('notification.flap_threshold_percent', '25') ON CONFLICT (key) DO NOTHING;

-- +goose Down
ALTER TABLE monitors DROP COLUMN IF EXISTS confirmation_threshold;
ALTER TABLE monitors DROP COLUMN IF EXISTS notification_cooldown_minutes;

DELETE FROM settings WHERE key IN (
    'notification.confirmation_threshold',
    'notification.cooldown_minutes',
    'notification.flap_detection_enabled',
    'notification.flap_window_checks',
    'notification.flap_threshold_percent'
);
