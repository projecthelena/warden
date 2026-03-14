-- +goose Up
INSERT INTO settings (key, value) VALUES ('notification.event.down.enabled', 'true') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value) VALUES ('notification.event.up.enabled', 'true') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value) VALUES ('notification.event.degraded.enabled', 'true') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value) VALUES ('notification.event.flapping.enabled', 'true') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value) VALUES ('notification.event.stabilized.enabled', 'true') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value) VALUES ('notification.event.ssl_expiring.enabled', 'true') ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM settings WHERE key IN (
    'notification.event.down.enabled',
    'notification.event.up.enabled',
    'notification.event.degraded.enabled',
    'notification.event.flapping.enabled',
    'notification.event.stabilized.enabled',
    'notification.event.ssl_expiring.enabled'
);
