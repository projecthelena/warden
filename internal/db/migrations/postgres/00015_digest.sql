-- +goose Up
INSERT INTO settings (key, value) VALUES ('notification.digest.enabled', 'false') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value) VALUES ('notification.digest.time', '09:00') ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value) VALUES ('notification.digest.event_types', 'degraded,flapping,stabilized,ssl_expiring') ON CONFLICT (key) DO NOTHING;

CREATE TABLE IF NOT EXISTS notification_digest_queue (
    id SERIAL PRIMARY KEY,
    monitor_id TEXT NOT NULL,
    monitor_name TEXT NOT NULL,
    monitor_url TEXT NOT NULL,
    event_type TEXT NOT NULL,
    message TEXT NOT NULL,
    event_time TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS notification_digest_queue;
DELETE FROM settings WHERE key IN (
    'notification.digest.enabled',
    'notification.digest.time',
    'notification.digest.event_types'
);
