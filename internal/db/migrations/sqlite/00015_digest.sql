-- +goose Up
INSERT OR IGNORE INTO settings (key, value) VALUES ('notification.digest.enabled', 'false');
INSERT OR IGNORE INTO settings (key, value) VALUES ('notification.digest.time', '09:00');
INSERT OR IGNORE INTO settings (key, value) VALUES ('notification.digest.event_types', 'degraded,flapping,stabilized,ssl_expiring');

CREATE TABLE IF NOT EXISTS notification_digest_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    monitor_id TEXT NOT NULL,
    monitor_name TEXT NOT NULL,
    monitor_url TEXT NOT NULL,
    event_type TEXT NOT NULL,
    message TEXT NOT NULL,
    event_time DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS notification_digest_queue;
DELETE FROM settings WHERE key IN (
    'notification.digest.enabled',
    'notification.digest.time',
    'notification.digest.event_types'
);
