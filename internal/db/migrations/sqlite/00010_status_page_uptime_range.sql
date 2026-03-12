-- +goose Up
ALTER TABLE status_pages ADD COLUMN uptime_days_range INTEGER DEFAULT 90;
UPDATE settings SET value = '365' WHERE key = 'data_retention_days' AND value = '30';

-- +goose Down
