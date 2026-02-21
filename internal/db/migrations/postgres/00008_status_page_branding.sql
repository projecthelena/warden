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
ALTER TABLE status_pages DROP COLUMN IF EXISTS description;
ALTER TABLE status_pages DROP COLUMN IF EXISTS logo_url;
ALTER TABLE status_pages DROP COLUMN IF EXISTS accent_color;
ALTER TABLE status_pages DROP COLUMN IF EXISTS theme;
ALTER TABLE status_pages DROP COLUMN IF EXISTS show_uptime_bars;
ALTER TABLE status_pages DROP COLUMN IF EXISTS show_uptime_percentage;
ALTER TABLE status_pages DROP COLUMN IF EXISTS show_incident_history;
