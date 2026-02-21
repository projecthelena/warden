-- +goose Up
-- Add new columns to incidents for approval workflow and outage linking
ALTER TABLE incidents ADD COLUMN source TEXT DEFAULT 'manual';
ALTER TABLE incidents ADD COLUMN outage_id INTEGER REFERENCES monitor_outages(id);
ALTER TABLE incidents ADD COLUMN public BOOLEAN DEFAULT FALSE;

-- Create incident updates table for timeline
CREATE TABLE IF NOT EXISTS incident_updates (
    id SERIAL PRIMARY KEY,
    incident_id TEXT NOT NULL,
    status TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(incident_id) REFERENCES incidents(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_incident_updates_incident_id ON incident_updates(incident_id);

-- +goose Down
DROP INDEX IF EXISTS idx_incident_updates_incident_id;
DROP TABLE IF EXISTS incident_updates;
ALTER TABLE incidents DROP COLUMN IF EXISTS public;
ALTER TABLE incidents DROP COLUMN IF EXISTS outage_id;
ALTER TABLE incidents DROP COLUMN IF EXISTS source;
