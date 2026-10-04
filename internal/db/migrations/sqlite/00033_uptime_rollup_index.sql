-- +goose Up
-- Cover the daily rollup and keep its GROUP BY in index order.
CREATE INDEX IF NOT EXISTS idx_monitor_checks_daily_rollup
    ON monitor_checks(monitor_id, DATE(timestamp), timestamp, status);

-- +goose Down
DROP INDEX IF EXISTS idx_monitor_checks_daily_rollup;
