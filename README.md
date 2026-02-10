# Warden

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
  -e ADMIN_SECRET=change-me \
  -v warden_data:/data \
  ghcr.io/projecthelena/warden:latest
```

Open `http://localhost:9090` and create your admin account using the secret above.

## Environment Variables

| Variable | Default | Description |
| :--- | :--- | :--- |
| `ADMIN_SECRET` | — | **Required.** Secret used to create the initial admin account. |
| `LISTEN_ADDR` | `:9090` | Address and port to listen on. |
| `DB_PATH` | `/data/warden.db` | Path to the SQLite database file. |
| `DB_TYPE` | `sqlite` | Database engine: `sqlite` or `postgres`. |
| `DB_URL` | — | PostgreSQL connection string (auto-sets `DB_TYPE`). |
| `COOKIE_SECURE` | `false` | Set `true` when serving over HTTPS. |
| `TRUST_PROXY` | `false` | Trust `X-Forwarded-For` headers. Enable only behind a reverse proxy. |

## Docker Compose

```yaml
services:
  warden:
    image: ghcr.io/projecthelena/warden:latest
    ports:
      - "9090:9090"
    environment:
      - ADMIN_SECRET=change-me
    volumes:
      - warden_data:/data

volumes:
  warden_data:
```

## Documentation

See the [`docs/`](docs/) folder for detailed guides:

- [API](docs/api.md) — REST API and Swagger docs
- [Database](docs/database.md) — SQLite vs PostgreSQL configuration
- [Load Testing](docs/load-testing.md) *(coming soon)*

## License

[AGPL-3.0](LICENSE) — Project Helena
