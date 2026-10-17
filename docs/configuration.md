# Configuration

Warden is configured with environment variables. The defaults run locally with SQLite and require no external services.

| Variable | Default | Description |
| :-- | :-- | :-- |
| `INCREMENTAL_UPTIME_ENABLED` | `false` | Opt in to transactional daily uptime counters on SQLite or PostgreSQL. Requires restart; set `false` to restore periodic rollups. See [mode switching](database.md#daily-uptime-feature-flag). |
| `LISTEN_ADDR` | `:9090` | Address and port Warden listens on. |
| `ROLLUP_DIAGNOSTICS_ENABLED` | `false` | Opt in to daily uptime rollup phase metrics and structured logs for runs lasting at least one second. Requires a restart. |
| `HTTP_DIAGNOSTICS_ENABLED` | `false` | Also record HTTP phases for legacy/manual-mode monitors; automatic retry mode already records its attempts. See [HTTP diagnostics](http-diagnostics.md). Requires a restart. |
| `SECONDARY_PROBE_URL` | empty (disabled) | Optional second probe endpoint over HTTPS or a loopback HTTP tunnel. Requires HTTP diagnostics. |
| `SECONDARY_PROBE_TOKEN` | empty | Shared probe API secret, at least 32 characters. Never sent to monitored targets. |
| `METRICS_ENABLED` | `true` | Enable Prometheus instrumentation and `/metrics`. Set `false` and restart to disable metric updates, diagnostic timing and rollup diagnostic logs. |
| `OBSERVABILITY_ADDR` | empty (disabled) | Separate address for Prometheus metrics and Go profiles, for example `127.0.0.1:9091`. |
| `DB_TYPE` | `sqlite` | Database backend: `sqlite` or `postgres`. A PostgreSQL `DB_URL` also selects PostgreSQL automatically. |
| `SQLITE_ROLLUP_BATCH_SIZE` | `50` | Monitors per daily uptime rollup read, SQLite only. Integer from 1 to 50; invalid values log a warning and use 50 without preventing startup. Requires a restart. |
| `DB_PATH` | `/data/warden.db` | SQLite database path. |
| `DB_URL` | — | PostgreSQL connection string, such as `postgres://user:pass@host:5432/warden`. |
| `COOKIE_SECURE` | `false` | Set to `true` when Warden is served over HTTPS so login cookies are never sent over plain HTTP. |
| `TRUST_PROXY` | `false` | Set to `true` behind a trusted reverse proxy so Warden can use the forwarded client IP for rate limiting. |
| `ADMIN_SECRET` | — | Development and test option that enables database reset and disables rate limits. Never set it in production. |

See [Database](database.md) for backend-specific setup and [Notifications](notifications.md) for notification channel configuration.

## Reverse proxies

Set `TRUST_PROXY=true` only when Warden can be reached exclusively through a trusted proxy such as Traefik, Caddy, or nginx. Otherwise a client can forge forwarding headers and evade IP-based rate limits.

## Metrics and profiling

Set `OBSERVABILITY_ADDR` to enable a separate diagnostics listener. It exposes Prometheus metrics at `/metrics` and Go profiles under `/debug/pprof/`. The listener has no authentication, so bind it to loopback or a private network and never publish it through the same ingress as Warden.

Set `METRICS_ENABLED=false` and restart to disable all Warden Prometheus updates and their timing clock reads, including baseline, persistence, HTTP, status-page and monitor metrics. The rollup diagnostic flag is overridden: rollups still execute, but without phase timing or slow-rollup diagnostic logs. Ordinary errors, monitoring, check latency used by alerts, and optional HTTP diagnostics remain unchanged. Disabled batch writes use the ordinary transaction path without separately acquiring a connection for measurement. Small flag checks and startup collector allocation remain; disabling metrics does not mean zero overhead.

With metrics disabled, `/metrics` returns 404 even if `OBSERVABILITY_ADDR` is set. The private listener can still serve `pprof`. Leave `OBSERVABILITY_ADDR` empty as well to disable that listener entirely. Conversely, leaving only the address empty does not stop in-process instrumentation. The switch is startup configuration, not a live API or UI toggle; set `METRICS_ENABLED=true` and restart to restore instrumentation. Invalid boolean values fail startup.

The metrics cover HTTP request rate, errors and latency; monitor check rate and latency; scheduling drops and queue depth; database batch size and write latency; and the active monitor count. Standard Go process and runtime metrics are included by the Prometheus client.

Set `COOKIE_SECURE=true` whenever the public Warden URL uses HTTPS.

### Baseline and check-write diagnostics

These metrics are available when `METRICS_ENABLED=true` on the diagnostics listener without enabling the rollup flag:

| Metric | Meaning |
| --- | --- |
| `warden_db_operation_duration_seconds{operation,outcome}` | Histogram of completed operations, including errors; outcomes are `success` and `error`. |
| `warden_db_operations_in_flight{operation}` | Active operations, including those waiting on the database. |
| `warden_db_operation_last_started_timestamp_seconds{operation}` | Unix timestamp of the latest start. With overlapping operations, this is not the oldest active call. |
| `warden_baseline_monitors_total{outcome}` | Monitors processed by the baseline worker: `updated`, `skipped` for insufficient samples, or `error`. |

Operations are fixed labels: `baseline_count`, `baseline_p50`, `baseline_p95`, `baseline_save`, `baseline_cycle`, `persist_connection`, `persist_begin`, `persist_insert`, and `persist_commit`. No SQL, monitor IDs or target URLs are exported. Baseline query timings include pool waits and result scanning. Check writes separate connection acquisition from transaction begin, prepared inserts (including preparation and cleanup), and commit. Unreached phases are not observed. The baseline cycle includes applying thresholds; its error outcome includes computation or threshold-loading failures. Cycle and per-query durations overlap and must not be summed together.

Scrape `/metrics` into Prometheus to retain history; Warden does not store these time series. A five-second scrape interval can help correlate load-test stalls with existing queue, pool-wait, rollup, HTTP and persistence metrics. Histograms record completed operations, so use the in-flight gauge to see unfinished work. For the serial baseline worker, elapsed time of its active cycle is:

```promql
(time() - warden_db_operation_last_started_timestamp_seconds{operation="baseline_cycle"})
  and on(operation) (warden_db_operations_in_flight{operation="baseline_cycle"} > 0)
```

For frequent operations, compare p95 duration by operation:

```promql
histogram_quantile(0.95,
  sum by (le, operation) (rate(warden_db_operation_duration_seconds_bucket[5m]))
)
```

The hourly baseline cycle has too few samples for a useful five-minute percentile; inspect its duration sum/count over a longer window and its active-cycle elapsed time instead. These are application wall times, not database-server execution plans or proof of a lock/I/O cause. Counters reset when Warden restarts.

### Rollup diagnostics

Set `ROLLUP_DIAGNOSTICS_ENABLED=true` on the Warden process to diagnose database contention. This flag is independent of `OBSERVABILITY_ADDR`: slow-operation logs work without the metrics listener; scraping requires both. Unset the flag or set it to `false` and restart to disable diagnostics. With `INCREMENTAL_UPTIME_ENABLED=false` (the default), daily rollups and their five-minute refresh remain active either way. Incremental mode replaces these runs with counters written in each check transaction, so this diagnostic flag does not measure the incremental path; use persistence metrics instead.

With diagnostics enabled, `warden_uptime_rollup_in_progress{mode}` marks active work and `warden_uptime_rollup_runs_total{mode,outcome}` counts completed runs. Modes are `backfill` and `refresh`; outcomes are `success` and `error`. `warden_uptime_rollup_rows{mode}` records aggregated monitor-day rows, which may not have committed if a write fails.

`warden_uptime_rollup_duration_seconds{mode,phase}` measures `read_connection`, `query`, `scan`, `write_connection`, `begin`, `upsert`, `commit` and `total`. Connection acquisition is separate from database execution. Drivers can defer database work until rows are read, so interpret query and scan together; scan also includes decoding and result cleanup. Phases not reached after an error are recorded as zero. Total includes cleanup and instrumentation overhead and need not equal the sum of phases.

Runs lasting at least one second produce one structured `slow uptime rollup` warning with phase durations, outcome, row count, and pool snapshots before/after. Pool wait counts and durations include all concurrent database users: their difference is correlation evidence, not time attributable exclusively to the rollup. The diagnostic log omits SQL, targets, credentials and driver error text. Existing rollup error logging is unchanged.

Smaller `SQLITE_ROLLUP_BATCH_SIZE` values (for example, `10`) release the SQLite connection more often between rollup reads, allowing queued checks and API requests to run. They also increase query overhead and may lengthen the complete refresh. Compare under representative load before changing the default. This setting does not change check-write batches or retention and is ignored with PostgreSQL. It does not affect incremental writes or the startup reconciliation when `INCREMENTAL_UPTIME_ENABLED=true`.
