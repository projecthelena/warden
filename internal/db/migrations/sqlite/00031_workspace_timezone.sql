-- +goose Up
INSERT OR IGNORE INTO settings (key, value)
VALUES ('workspace.timezone', COALESCE((SELECT timezone FROM users ORDER BY id LIMIT 1), 'UTC'));

-- +goose Down
DELETE FROM settings WHERE key = 'workspace.timezone';
