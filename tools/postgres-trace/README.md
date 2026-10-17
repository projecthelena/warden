# PostgreSQL operation traces

`POSTGRES_TRACE_ENABLED=true` enables diagnostic `db_trace` JSON records in Warden's normal logs for PostgreSQL connections. It is off by default, independent of `METRICS_ENABLED`, and has no effect on SQLite. Restart Warden to change it. Use a bounded diagnostic window: logging every operation adds CPU, disk and log volume. Measure the overhead before comparing traced and untraced load tests.

Each operation has start/end records, UTC timestamps, duration, a random process `run`, physical `connection`, PostgreSQL backend `pid`, and `span`. Transactions have a shared `transaction` identifier; this is a Warden identifier, not a PG XID. No SQL, arguments, URLs, monitor identifiers or error messages are recorded. Errors expose only SQLSTATE when available. Fixed operation labels distinguish check inserts, daily-counter upserts, counts and percentiles.

SQL sent to PostgreSQL carries `/*warden:<run>:<span>*/`. For prepared statements, that tag identifies the preparation; execution events carry it as `statement` and have their own span. Match these by run, PID and time. BEGIN/COMMIT/ROLLBACK are correlated by PID, connection, transaction and time, without SQL comments. Query duration includes consuming or closing rows; it is not server execution time alone. Pool acquisition remains visible through existing metrics; driver spans start after acquiring a physical connection. A single backend-PID query is added when a new connection opens, not for every operation.

Run these samplers in external clients connected to the **same diagnostic database**, using credentials from a protected environment or `.pgpass`:

```sh
python3 tools/postgres-trace/capture.py activity activity.jsonl.gz --seconds 600
# In another terminal:
python3 tools/postgres-trace/capture.py stats stats.jsonl.gz --seconds 600
```

The capture command refuses to overwrite files, stops its external psql client at the deadline or on SIGTERM/SIGINT, and closes the gzip stream. Credentials come from standard libpq environment variables or `.pgpass`, never command arguments.

The activity sampler runs every 250 ms and records the Warden tag, query ID, backend lifetime, wait event, blockers and waiting locks, without SQL text or parameters. Statistics run every second. PG statistics can lag; lack of change in a blocked interval does not prove zero physical disk activity. Collect host I/O and PSI alongside them. Sampling can miss operations shorter than 250 ms; the Warden start/end log still records those operations.

For native PostgreSQL duration logs, a DBA can set `log_duration=on` for the application role **in the diagnostic database** and reconnect the application. Save the existing per-role/per-database settings before changing them. Keep the server log prefix's timestamp and PID. Use `compute_query_id=on`, `track_io_timing=on` and `track_wal_io_timing=on` if permitted. Query IDs are normalized identifiers, not Warden span IDs.

Do not enable `log_statement` or `log_min_duration_statement` to collect these traces: those can expose SQL and values. Set `log_parameter_max_length=0` and `log_parameter_max_length_on_error=0` for the diagnostic role and verify other statement logging settings do not override the intended policy. `log_duration` records separate parse/bind/execute durations for extended protocol; do not sum them as independent application queries. Existing database error messages may still contain data; restrict access to server logs.

Stop samplers at the end of the window, flush/close compressed output, archive logs and restore exactly the saved settings. Turning off Warden tracing does not disable PostgreSQL logging. Do not restart production or modify its logging configuration for a load-test investigation.

References: [PostgreSQL logging](https://www.postgresql.org/docs/18/runtime-config-logging.html) and [activity/statistics](https://www.postgresql.org/docs/18/monitoring-stats.html).

## Central storage

Trace events are JSON lines (`event=db_trace`) with `trace_id` and `span_id` when OTLP export is configured. Container log collectors can send these events to VictoriaLogs together with PostgreSQL container logs and sampler output.

```sh
POSTGRES_TRACE_ENABLED=true
OTEL_SERVICE_NAME=warden
OTEL_RESOURCE_ATTRIBUTES=deployment.environment.name=load-test
OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://victoria-traces.observability.svc.cluster.local:10428/insert/opentelemetry/v1/traces
```

VictoriaTraces receives client spans for database operations and their enclosing transactions. PostgreSQL itself does not emit OTLP spans: match its activity samples using backend PID and `warden:<run>:<span>` SQL comments. The corresponding span attributes are `db.postgresql.backend_pid`, `warden.run`, and `warden.span`; prepared executions also carry `warden.statement`.

Export runs in the background with a bounded 2,048-span queue. A full queue drops spans rather than blocking database writes. Export and shutdown have five-second timeouts. This is diagnostic telemetry, not a lossless audit log. No SQL text, parameters or database error messages are exported. Instrumentation adds work; measure its effect before comparing load-test results.

Set `POSTGRES_TRACE_ENABLED=false` and restart to disable both JSON tracing and OTLP export. Removing just the endpoint retains JSON tracing. PostgreSQL native logging and external samplers have their own independent switches.

Use `capture.py activity /data/activity.jsonl.gz --seconds 600 --stdout` in a collector-scoped pod to retain the gzip artifact and emit structured samples to VictoriaLogs. Samples carry `event=postgres_activity` or `postgres_stats`. Local archives are not automatically uploaded. Keep the sampler on a separate node from PostgreSQL and bound its duration; it is independent of the Warden flag.
