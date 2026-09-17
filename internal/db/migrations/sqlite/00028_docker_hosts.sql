-- +goose Up
CREATE TABLE docker_hosts (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    endpoint TEXT NOT NULL,
    tls_verify BOOLEAN NOT NULL DEFAULT FALSE,
    ca_cert TEXT NOT NULL DEFAULT '',
    client_cert TEXT NOT NULL DEFAULT '',
    client_key TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE docker_hosts;
