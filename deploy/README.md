# Deploy

## SQLite (default)

```bash
docker compose -f docker-compose.sqlite.yml up -d
```

Zero setup. Data stored in a single file. Best for most users.

## PostgreSQL

```bash
docker compose -f docker-compose.postgres.yml up -d
```

External database. Best when you already run PostgreSQL or need to scale beyond a single instance.

## Which one should I pick?

| | SQLite | PostgreSQL |
| :--- | :--- | :--- |
| Setup | Nothing to configure | Requires a running PostgreSQL server |
| Best for | Single instance, low-to-medium traffic | High availability, large-scale monitoring |
| Backups | Copy one file | Use `pg_dump` or your existing backup pipeline |
| Performance | Great for most workloads | Better under heavy concurrent writes |

When in doubt, start with SQLite. You can migrate later if needed.
