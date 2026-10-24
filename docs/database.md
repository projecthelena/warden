# Database

Warden supports two database backends: **SQLite** (default) and **PostgreSQL**.

## SQLite (default)

Zero configuration. Data is stored in a single file.

```bash
docker run -d -p 9090:9090 \
  -v warden_data:/data \
  ghcr.io/projecthelena/warden:latest
```

| Variable  | Default           | Description              |
| :-------- | :---------------- | :----------------------- |
| `DB_PATH` | `/data/warden.db` | Path to the SQLite file. |

**When to use:** Single-instance deployments, low-to-medium traffic, simplicity.

## PostgreSQL

For larger deployments or when you need an external database.

```bash
docker run -d -p 9090:9090 \
  -e DB_URL=postgres://user:password@db-host:5432/warden?sslmode=disable \
  ghcr.io/projecthelena/warden:latest
```

| Variable | Default | Description |
| :-- | :-- | :-- |
| `DB_TYPE` | `sqlite` | Set to `postgres` (auto-detected if `DB_URL` starts with `postgres`). |
| `DB_URL` | — | PostgreSQL connection string. |

**When to use:** High availability requirements, existing PostgreSQL infrastructure, large-scale monitoring.

### Check durability

Batches of check results commit with `synchronous_commit = off`, so a slow disk flush does not stall ingestion. If the PostgreSQL server crashes, the most recently committed checks (normally under a second, at most three times `wal_writer_delay`) can be lost; each batch and its daily uptime counters are kept or lost together, and the database stays consistent. Events, outages, incidents and settings keep synchronous commits. Warden's own in-memory queue was never durable.

### Settings for large installations

With thousands of monitors, most write volume is write-ahead log rather than check rows. These server settings reduce it without changing durability:

| Setting | Suggested | Why |
| :-- | :-- | :-- |
| `wal_compression` | `lz4` | Compresses the full-page images that index writes log after each checkpoint. |
| `max_wal_size` | `8GB` or more | Fewer checkpoints, so fewer full-page images. Needs that much free disk for WAL. |

For example, with the official image: `postgres -c wal_compression=lz4 -c max_wal_size=8GB`.

## Migrating from SQLite to PostgreSQL

There is no built-in migration tool between backends. To migrate:

1. Export your SQLite data
2. Import into PostgreSQL
3. Update environment variables to point to PostgreSQL

## Moving an existing PostgreSQL installation

Follow [Move Warden to another PostgreSQL instance](postgresql-migration.md) for the complete dump, restore, verification, rollback, and latency-relearning checklist.

## Daily uptime feature flag

`INCREMENTAL_UPTIME_ENABLED` defaults to `false` on SQLite and PostgreSQL. The default keeps the existing writer, startup backfill and five-minute rollup. Set it to `true` and restart to persist daily counters with each batch of checks in one transaction, with stable identities and safe retries. No external queue or database extension is required.

| Mode | Writes | Background aggregation |
| :-- | :-- | :-- |
| `false` (default) | Existing check writer and error handling | Startup backfill and five-minute refresh |
| `true` | Batched check inserts and daily increments in one transaction | No periodic rollup; reconciliation only on first activation or reactivation |

To enable: back up the database, stop all Warden instances that write to it, set `INCREMENTAL_UPTIME_ENABLED=true`, and restart. Startup reconstructs summaries from available checks and records completion in the same transaction. The application does not start ingestion if reconciliation fails. Normal restarts in incremental mode skip that reconstruction. Allow sufficient startup time for this scan when enabling the flag on a large database.

To return to the previous behavior: stop the writers, set `INCREMENTAL_UPTIME_ENABLED=false`, and restart the same binary. This restores the previous write path and rollup worker and invalidates the incremental initialization marker. No schema downgrade or data deletion is needed. If you later enable the flag again, startup reconciles checks accumulated while it was off before resuming incremental writes. Keep all writers on the same mode; mixed-mode rolling changes are unsupported.

Schema migration 00037 adds the optional identity column and a partial unique index in both modes; legacy writes leave that identity empty. Reconciliation happens only when enabling the flag and can take time on large histories. Existing summaries at or before the partially retained calendar day are preserved; missing summaries can only be built from raw data still available. Expired checks cannot be recovered.

The reconstruction window uses the configured retention period. If retention was increased after older checks had already been deleted, audit history before enabling: the current window alone cannot establish which days remain complete. Use the feature flag for rollback; running an older binary would not invalidate the initialization marker and requires separate reconciliation before incremental mode can resume safely.

In incremental mode, temporary persistence errors retain the current batch for retry. A permanent error stops check persistence and logs the failure; restart after correcting its cause. Deleting a monitor discards its pending results without blocking other monitors. Results still waiting in memory are not durable in either mode. Normal shutdown waits up to ten seconds for in-flight checks and queued results, logging if the deadline expires.
