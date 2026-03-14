-- +goose Up
INSERT INTO settings (key, value) VALUES ('notification.recovery_confirmation_checks', '1') ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM settings WHERE key = 'notification.recovery_confirmation_checks';
