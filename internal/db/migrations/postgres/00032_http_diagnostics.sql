-- +goose Up
ALTER TABLE monitor_checks ADD COLUMN diagnostics TEXT;
ALTER TABLE monitor_events ADD COLUMN diagnostics TEXT;

CREATE INDEX idx_monitor_checks_monitor_id_id ON monitor_checks(monitor_id, id DESC);

-- +goose Down
DROP INDEX idx_monitor_checks_monitor_id_id;
ALTER TABLE monitor_events DROP COLUMN diagnostics;
ALTER TABLE monitor_checks DROP COLUMN diagnostics;
