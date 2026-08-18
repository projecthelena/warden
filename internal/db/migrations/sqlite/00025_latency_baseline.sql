-- +goose Up
-- Rolling latency percentiles per monitor. Persisted rather than kept in memory so a
-- restart does not spend an hour with no idea what "normal" looks like for each target.
CREATE TABLE IF NOT EXISTS monitor_latency_baseline (
    monitor_id  TEXT PRIMARY KEY,
    p50         INTEGER NOT NULL,
    p95         INTEGER NOT NULL,
    samples     INTEGER NOT NULL,
    computed_at TIMESTAMP NOT NULL,
    FOREIGN KEY(monitor_id) REFERENCES monitors(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE IF EXISTS monitor_latency_baseline;
