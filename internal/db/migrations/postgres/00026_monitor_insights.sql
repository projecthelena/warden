-- +goose Up
-- Findings from the pattern detectors. Recomputed daily and replaced wholesale per
-- monitor, so a pattern that stops happening stops being reported.
CREATE TABLE IF NOT EXISTS monitor_insights (
    id          SERIAL PRIMARY KEY,
    monitor_id  TEXT NOT NULL,
    kind        TEXT NOT NULL,
    summary     TEXT NOT NULL,
    detail      TEXT,
    confidence  TEXT NOT NULL,
    detected_at TIMESTAMP NOT NULL,
    FOREIGN KEY(monitor_id) REFERENCES monitors(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_monitor_insights_monitor ON monitor_insights(monitor_id);

-- +goose Down
DROP INDEX IF EXISTS idx_monitor_insights_monitor;
DROP TABLE IF EXISTS monitor_insights;
