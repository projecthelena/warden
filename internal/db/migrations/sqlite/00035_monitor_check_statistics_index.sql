-- +goose Up
-- PostgreSQL-only covering index; SQLite retains its existing rollup index.
SELECT 1;

-- +goose Down
SELECT 1;
