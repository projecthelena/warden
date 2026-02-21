-- +goose Up
-- Add branding and display configuration to status pages
ALTER TABLE status_pages ADD COLUMN description TEXT DEFAULT '';
ALTER TABLE status_pages ADD COLUMN logo_url TEXT DEFAULT '';
ALTER TABLE status_pages ADD COLUMN accent_color TEXT DEFAULT '';
ALTER TABLE status_pages ADD COLUMN theme TEXT DEFAULT 'system';
ALTER TABLE status_pages ADD COLUMN show_uptime_bars BOOLEAN DEFAULT TRUE;
ALTER TABLE status_pages ADD COLUMN show_uptime_percentage BOOLEAN DEFAULT TRUE;
ALTER TABLE status_pages ADD COLUMN show_incident_history BOOLEAN DEFAULT TRUE;

-- +goose Down
-- SQLite doesn't support DROP COLUMN, so we leave the columns in place for down migration
