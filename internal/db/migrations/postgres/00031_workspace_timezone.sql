-- +goose Up
INSERT INTO settings (key, value)
VALUES ('workspace.timezone', COALESCE((SELECT timezone FROM users ORDER BY id LIMIT 1), 'UTC'))
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM settings WHERE key = 'workspace.timezone';
