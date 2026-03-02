-- +goose Up
-- Add new columns to incidents for approval workflow and outage linking
ALTER TABLE incidents ADD COLUMN source TEXT DEFAULT 'manual';
ALTER TABLE incidents ADD COLUMN outage_id INTEGER REFERENCES monitor_outages(id);
ALTER TABLE incidents ADD COLUMN public BOOLEAN DEFAULT FALSE;

-- Create incident updates table for timeline
CREATE TABLE IF NOT EXISTS incident_updates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    incident_id TEXT NOT NULL,
    status TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(incident_id) REFERENCES incidents(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_incident_updates_incident_id ON incident_updates(incident_id);

-- +goose Down
DROP INDEX IF EXISTS idx_incident_updates_incident_id;
DROP TABLE IF EXISTS incident_updates;
-- SQLite doesn't support DROP COLUMN, so we leave the columns in place for down migration
