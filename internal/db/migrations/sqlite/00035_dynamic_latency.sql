-- +goose Up
-- Fixed millisecond overrides cannot be converted without a trusted baseline.
-- Keep the old column for rollback; monitors inherit the percentage by default.
ALTER TABLE monitors ADD COLUMN latency_increase_percent INTEGER CHECK (latency_increase_percent BETWEEN 1 AND 9900);
INSERT INTO settings (key, value)
SELECT 'notification.latency.increase_percent', CAST(CAST(value AS INTEGER) - 100 AS TEXT)
FROM settings WHERE key = 'notification.latency.factor_percent' AND CAST(value AS INTEGER) BETWEEN 101 AND 10000
ON CONFLICT (key) DO NOTHING;
INSERT INTO settings (key, value) VALUES ('notification.latency.increase_percent', '50') ON CONFLICT (key) DO NOTHING;

-- +goose Down
ALTER TABLE monitors DROP COLUMN latency_increase_percent;
DELETE FROM settings WHERE key = 'notification.latency.increase_percent';
