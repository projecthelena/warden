-- +goose Up
-- History pages by (timestamp, id) on idx_monitor_checks_statistics. This index kept
-- one hot leaf page per monitor, costing a full page image per check after every
-- checkpoint.
DROP INDEX idx_monitor_checks_monitor_id_id;

-- +goose Down
CREATE INDEX idx_monitor_checks_monitor_id_id ON monitor_checks(monitor_id, id DESC);
