# Database

Warden supports two database backends: **SQLite** (default) and **PostgreSQL**.

## SQLite (default)

Zero configuration. Data is stored in a single file.

```bash
docker run -d -p 9090:9090 \
  -e ADMIN_SECRET=change-me \
  -v warden_data:/data \
  ghcr.io/projecthelena/warden:latest
```

| Variable | Default | Description |
| :--- | :--- | :--- |
| `DB_PATH` | `/data/warden.db` | Path to the SQLite file. |

**When to use:** Single-instance deployments, low-to-medium traffic, simplicity.

## PostgreSQL

For larger deployments or when you need an external database.

```bash
docker run -d -p 9090:9090 \
  -e ADMIN_SECRET=change-me \
  -e DB_URL=postgres://user:password@db-host:5432/warden?sslmode=disable \
  ghcr.io/projecthelena/warden:latest
```

| Variable | Default | Description |
| :--- | :--- | :--- |
| `DB_TYPE` | `sqlite` | Set to `postgres` (auto-detected if `DB_URL` starts with `postgres`). |
| `DB_URL` | — | PostgreSQL connection string. |

**When to use:** High availability requirements, existing PostgreSQL infrastructure, large-scale monitoring.

## Migrating from SQLite to PostgreSQL

There is no built-in migration tool between backends. To migrate:

1. Export your SQLite data
2. Import into PostgreSQL
3. Update environment variables to point to PostgreSQL
