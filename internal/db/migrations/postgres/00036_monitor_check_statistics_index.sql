-- +goose Up
-- Keep the history lookup order while allowing maintenance queries to read
-- status and latency without fetching every check's diagnostics from the heap.
CREATE INDEX idx_monitor_checks_statistics
    ON monitor_checks (monitor_id, timestamp DESC) INCLUDE (status, latency);
DROP INDEX idx_monitor_checks_monitor_id_ts;

-- +goose Down
CREATE INDEX idx_monitor_checks_monitor_id_ts ON monitor_checks (monitor_id, timestamp DESC);
DROP INDEX idx_monitor_checks_statistics;
