-- +goose Up
CREATE TABLE sso_providers (
    id TEXT PRIMARY KEY,
    template TEXT NOT NULL,
    name TEXT NOT NULL,
    issuer_url TEXT NOT NULL,
    client_id TEXT NOT NULL,
    client_secret TEXT NOT NULL,
    allowed_domains TEXT NOT NULL DEFAULT '',
    auto_provision BOOLEAN NOT NULL DEFAULT TRUE,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO sso_providers (id, template, name, issuer_url, client_id, client_secret, allowed_domains, auto_provision, enabled, sort_order)
SELECT 'google', 'google', 'Google', 'https://accounts.google.com',
       COALESCE((SELECT value FROM settings WHERE key = 'sso.google.client_id'), ''),
       COALESCE((SELECT value FROM settings WHERE key = 'sso.google.client_secret'), ''),
       COALESCE((SELECT value FROM settings WHERE key = 'sso.google.allowed_domains'), ''),
       COALESCE((SELECT value FROM settings WHERE key = 'sso.google.auto_provision'), 'true') = 'true',
       COALESCE((SELECT value FROM settings WHERE key = 'sso.google.enabled'), 'false') = 'true', 0
WHERE EXISTS (SELECT 1 FROM settings WHERE key LIKE 'sso.google.%');

INSERT INTO sso_providers (id, template, name, issuer_url, client_id, client_secret, allowed_domains, auto_provision, enabled, sort_order)
SELECT 'oidc', 'oidc', COALESCE(NULLIF((SELECT value FROM settings WHERE key = 'sso.oidc.provider_name'), ''), 'OpenID Connect'),
       COALESCE((SELECT value FROM settings WHERE key = 'sso.oidc.issuer_url'), ''),
       COALESCE((SELECT value FROM settings WHERE key = 'sso.oidc.client_id'), ''),
       COALESCE((SELECT value FROM settings WHERE key = 'sso.oidc.client_secret'), ''),
       COALESCE((SELECT value FROM settings WHERE key = 'sso.oidc.allowed_domains'), ''),
       COALESCE((SELECT value FROM settings WHERE key = 'sso.oidc.auto_provision'), 'true') = 'true',
       COALESCE((SELECT value FROM settings WHERE key = 'sso.oidc.enabled'), 'false') = 'true', 1
WHERE EXISTS (SELECT 1 FROM settings WHERE key LIKE 'sso.oidc.%');

-- +goose Down
DROP TABLE sso_providers;
