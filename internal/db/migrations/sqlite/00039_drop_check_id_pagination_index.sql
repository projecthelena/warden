-- +goose Up
-- History pages by (timestamp, id) on idx_monitor_checks_monitor_id_ts.
DROP INDEX idx_monitor_checks_monitor_id_id;

-- +goose Down
CREATE INDEX idx_monitor_checks_monitor_id_id ON monitor_checks(monitor_id, id DESC);
