-- +goose Up
CREATE TABLE IF NOT EXISTS monitor_uptime_daily (
    monitor_id TEXT NOT NULL,
    day        TEXT NOT NULL,
    total      INTEGER NOT NULL,
    up_count   INTEGER NOT NULL,
    PRIMARY KEY (monitor_id, day),
    FOREIGN KEY(monitor_id) REFERENCES monitors(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_monitor_uptime_daily_day ON monitor_uptime_daily(day);

-- +goose Down
DROP TABLE IF EXISTS monitor_uptime_daily;
