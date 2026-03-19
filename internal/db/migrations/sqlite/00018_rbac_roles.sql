-- +goose Up
ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'admin';
ALTER TABLE api_keys ADD COLUMN role TEXT NOT NULL DEFAULT 'editor';
CREATE TABLE user_status_pages (
    user_id INTEGER NOT NULL,
    status_page_id INTEGER NOT NULL,
    PRIMARY KEY (user_id, status_page_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (status_page_id) REFERENCES status_pages(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE IF EXISTS user_status_pages;
