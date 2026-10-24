-- +goose Up
-- PostgreSQL-only autovacuum tuning; SQLite has no visibility map.
SELECT 1;

-- +goose Down
SELECT 1;
