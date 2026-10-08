# Configuration

Warden is configured with environment variables. The defaults run locally with SQLite and require no external services.

| Variable | Default | Description |
| :-- | :-- | :-- |
| `LISTEN_ADDR` | `:9090` | Address and port Warden listens on. |
| `ROLLUP_DIAGNOSTICS_ENABLED` | `false` | Opt in to daily uptime rollup phase metrics and structured logs for runs lasting at least one second. Requires a restart. |
| `HTTP_DIAGNOSTICS_ENABLED` | `false` | Also record HTTP phases for legacy/manual-mode monitors; automatic retry mode already records its attempts. See [HTTP diagnostics](http-diagnostics.md). Requires a restart. |
| `SECONDARY_PROBE_URL` | empty (disabled) | Optional second probe endpoint over HTTPS or a loopback HTTP tunnel. Requires HTTP diagnostics. |
| `SECONDARY_PROBE_TOKEN` | empty | Shared probe API secret, at least 32 characters. Never sent to monitored targets. |
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

The metrics cover HTTP request rate, errors and latency; monitor check rate and latency; scheduling drops and queue depth; database batch size and write latency; and the active monitor count. Standard Go process and runtime metrics are included by the Prometheus client.

Set `COOKIE_SECURE=true` whenever the public Warden URL uses HTTPS.

### Rollup diagnostics

Set `ROLLUP_DIAGNOSTICS_ENABLED=true` on the Warden process to diagnose database contention. This flag is independent of `OBSERVABILITY_ADDR`: slow-operation logs work without the metrics listener; scraping requires both. Unset the flag or set it to `false` and restart to disable diagnostics. Daily rollups, their five-minute refresh and status-page history remain active either way.

With diagnostics enabled, `warden_uptime_rollup_in_progress{mode}` marks active work and `warden_uptime_rollup_runs_total{mode,outcome}` counts completed runs. Modes are `backfill` and `refresh`; outcomes are `success` and `error`. `warden_uptime_rollup_rows{mode}` records aggregated monitor-day rows, which may not have committed if a write fails.

`warden_uptime_rollup_duration_seconds{mode,phase}` measures `read_connection`, `query`, `scan`, `write_connection`, `begin`, `upsert`, `commit` and `total`. Connection acquisition is separate from database execution. Drivers can defer database work until rows are read, so interpret query and scan together; scan also includes decoding and result cleanup. Phases not reached after an error are recorded as zero. Total includes cleanup and instrumentation overhead and need not equal the sum of phases.

Runs lasting at least one second produce one structured `slow uptime rollup` warning with phase durations, outcome, row count, and pool snapshots before/after. Pool wait counts and durations include all concurrent database users: their difference is correlation evidence, not time attributable exclusively to the rollup. The diagnostic log omits SQL, targets, credentials and driver error text. Existing rollup error logging is unchanged.

Smaller `SQLITE_ROLLUP_BATCH_SIZE` values (for example, `10`) release the SQLite connection more often between rollup reads, allowing queued checks and API requests to run. They also increase query overhead and may lengthen the complete refresh. Compare under representative load before changing the default. This setting does not change check-write batches or retention and is ignored with PostgreSQL.
