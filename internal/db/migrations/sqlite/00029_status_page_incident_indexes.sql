-- +goose Up
CREATE INDEX IF NOT EXISTS idx_incidents_active_created
    ON incidents(created_at DESC)
    WHERE status != 'resolved' AND status != 'completed';
CREATE INDEX IF NOT EXISTS idx_incidents_public_history
    ON incidents(start_time DESC)
    WHERE public = TRUE AND type = 'incident' AND status IN ('resolved', 'completed');

-- +goose Down
DROP INDEX IF EXISTS idx_incidents_public_history;
DROP INDEX IF EXISTS idx_incidents_active_created;
