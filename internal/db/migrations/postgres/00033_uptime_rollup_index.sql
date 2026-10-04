-- +goose Up
-- SQLite-only query-plan optimization; PostgreSQL keeps its existing indexes.
SELECT 1;

-- +goose Down
SELECT 1;
