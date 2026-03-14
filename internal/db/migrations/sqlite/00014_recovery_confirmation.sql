-- +goose Up
INSERT OR IGNORE INTO settings (key, value) VALUES ('notification.recovery_confirmation_checks', '1');

-- +goose Down
DELETE FROM settings WHERE key = 'notification.recovery_confirmation_checks';
