-- +goose Up
-- Add SSO fields to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS email TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS sso_provider TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS sso_id TEXT;

-- Create indexes for SSO lookups
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_sso ON users(sso_provider, sso_id) WHERE sso_provider IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email) WHERE email IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_users_sso;
DROP INDEX IF EXISTS idx_users_email;
ALTER TABLE users DROP COLUMN IF EXISTS sso_id;
ALTER TABLE users DROP COLUMN IF EXISTS sso_provider;
ALTER TABLE users DROP COLUMN IF EXISTS email;
