-- +goose Up
-- Add SSO fields to users table
ALTER TABLE users ADD COLUMN email TEXT;
ALTER TABLE users ADD COLUMN sso_provider TEXT;
ALTER TABLE users ADD COLUMN sso_id TEXT;

-- Create indexes for SSO lookups
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_sso ON users(sso_provider, sso_id) WHERE sso_provider IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email) WHERE email IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_users_sso;
DROP INDEX IF EXISTS idx_users_email;
-- Note: SQLite doesn't support DROP COLUMN directly
-- Columns will remain but be unused after rollback
