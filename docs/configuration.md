# Configuration

Warden is configured with environment variables. The defaults run locally with SQLite and require no external services.

| Variable | Default | Description |
| :-- | :-- | :-- |
| `LISTEN_ADDR` | `:9090` | Address and port Warden listens on. |
| `DB_TYPE` | `sqlite` | Database backend: `sqlite` or `postgres`. A PostgreSQL `DB_URL` also selects PostgreSQL automatically. |
| `DB_PATH` | `/data/warden.db` | SQLite database path. |
| `DB_URL` | — | PostgreSQL connection string, such as `postgres://user:pass@host:5432/warden`. |
| `COOKIE_SECURE` | `false` | Set to `true` when Warden is served over HTTPS so login cookies are never sent over plain HTTP. |
| `TRUST_PROXY` | `false` | Set to `true` behind a trusted reverse proxy so Warden can use the forwarded client IP for rate limiting. |
| `ADMIN_SECRET` | — | Development and test option that enables database reset and disables rate limits. Never set it in production. |

See [Database](database.md) for backend-specific setup and [Notifications](notifications.md) for notification channel configuration.

## Reverse proxies

Set `TRUST_PROXY=true` only when Warden can be reached exclusively through a trusted proxy such as Traefik, Caddy, or nginx. Otherwise a client can forge forwarding headers and evade IP-based rate limits.

Set `COOKIE_SECURE=true` whenever the public Warden URL uses HTTPS.
