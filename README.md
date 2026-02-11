# <img src="assets/favicon.svg" width="28" height="28" alt="PH" /> Warden

[![CI](https://github.com/projecthelena/warden/actions/workflows/ci.yml/badge.svg)](https://github.com/projecthelena/warden/actions/workflows/ci.yml)
[![Docker](https://github.com/projecthelena/warden/actions/workflows/docker.yml/badge.svg)](https://github.com/projecthelena/warden/actions/workflows/docker.yml)
[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)

Self-hosted uptime monitoring by [Project Helena](https://projecthelena.com). Single binary, no external dependencies.

<div align="center">
  <img src="assets/dashboard-overview.png" alt="Dashboard Preview" width="100%" />
</div>

## Quick Start

```bash
docker run -d -p 9090:9090 \
  -v warden_data:/data \
  ghcr.io/projecthelena/warden:latest
```

Open `http://localhost:9090` and create your admin account.

## Environment Variables

| Variable | Default | Description |
| :--- | :--- | :--- |
| `LISTEN_ADDR` | `:9090` | Port Warden listens on. Change it if 9090 is already taken. |
| `DB_TYPE` | `sqlite` | `sqlite` or `postgres`. Warden uses SQLite by default — no setup needed. Set to `postgres` if you want to use PostgreSQL. Takes precedence over `DB_URL` auto-detection. |
| `DB_PATH` | `/data/warden.db` | Where the SQLite database file is stored. Only matters when using SQLite. |
| `DB_URL` | — | PostgreSQL connection string (e.g. `postgres://user:pass@host:5432/warden`). Setting this automatically switches to PostgreSQL. |
| `COOKIE_SECURE` | `false` | Set `true` if you serve Warden over HTTPS. Tells browsers to only send login cookies over secure connections, preventing them from leaking on plain HTTP. |
| `TRUST_PROXY` | `false` | Set `true` if Warden runs behind a reverse proxy (nginx, Traefik, Caddy). Lets Warden see users' real IPs for rate limiting. Leave `false` if Warden is exposed directly — otherwise anyone can fake their IP. |
| `ADMIN_SECRET` | — | For development and testing only. Enables the database reset endpoint and disables rate limits. Do not set in production. |

## Docker Compose

Ready-to-use compose files in [`deploy/`](deploy/):

- [**SQLite**](deploy/docker-compose.sqlite.yml) — simplest, no extra services
- [**PostgreSQL**](deploy/docker-compose.postgres.yml) — for larger deployments

## Documentation

See the [`docs/`](docs/) folder for detailed guides:

- [API](docs/api.md) — REST API and Swagger docs
- [Database](docs/database.md) — SQLite vs PostgreSQL configuration
- [Load Testing](docs/load-testing.md) *(coming soon)*

## License

[AGPL-3.0](LICENSE) — Project Helena
