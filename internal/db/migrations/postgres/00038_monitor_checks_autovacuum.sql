-- +goose Up
-- Vacuum new checks every few thousand rows instead of after 20% of the table.
-- Until vacuum marks a page all-visible, index-only scans on
-- idx_monitor_checks_statistics still fetch every row from the heap, and with
-- data checksums each fetch that sets a hint bit logs a full page image. The
-- hourly baseline refresh then wrote ~9 GB of WAL at 5,000 monitors, starving
-- check persistence of fsync bandwidth.
ALTER TABLE monitor_checks SET (
    autovacuum_vacuum_insert_threshold = 10000,
    autovacuum_vacuum_insert_scale_factor = 0
);

-- +goose Down
ALTER TABLE monitor_checks RESET (
    autovacuum_vacuum_insert_threshold,
    autovacuum_vacuum_insert_scale_factor
);
