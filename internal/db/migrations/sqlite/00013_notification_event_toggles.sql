-- +goose Up
INSERT OR IGNORE INTO settings (key, value) VALUES ('notification.event.down.enabled', 'true');
INSERT OR IGNORE INTO settings (key, value) VALUES ('notification.event.up.enabled', 'true');
INSERT OR IGNORE INTO settings (key, value) VALUES ('notification.event.degraded.enabled', 'true');
INSERT OR IGNORE INTO settings (key, value) VALUES ('notification.event.flapping.enabled', 'true');
INSERT OR IGNORE INTO settings (key, value) VALUES ('notification.event.stabilized.enabled', 'true');
INSERT OR IGNORE INTO settings (key, value) VALUES ('notification.event.ssl_expiring.enabled', 'true');

-- +goose Down
DELETE FROM settings WHERE key IN (
    'notification.event.down.enabled',
    'notification.event.up.enabled',
    'notification.event.degraded.enabled',
    'notification.event.flapping.enabled',
    'notification.event.stabilized.enabled',
    'notification.event.ssl_expiring.enabled'
);
