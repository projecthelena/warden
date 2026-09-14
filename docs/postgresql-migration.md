# Move Warden to another PostgreSQL instance

Use this runbook when Warden is moving to another server, network, region, or homelab and its PostgreSQL database must move with it.

The process keeps monitors, groups, users, settings, incidents, API keys, status pages, and check history. The last step tells Warden to learn latency again from the new location, because network latency from the old location is no longer a valid baseline.

## Before you start

You need:

- the source and destination PostgreSQL connection URLs;
- `pg_dump`, `pg_restore`, `psql`, and `curl`;
- permission to stop both Warden instances;
- the exact Warden image version currently running on the source.

Use a `pg_dump` client from the same PostgreSQL major version as the source server, or a newer one. Never put a real password in this document or commit one to Git.

Set these variables in your terminal:

```bash
export SOURCE_DB_URL='postgres://warden@old-db.example.com:5432/warden?sslmode=require'
export DESTINATION_DB_URL='postgres://warden@new-db.example.com:5432/warden?sslmode=require'
export WARDEN_URL='https://status.example.com'
export WARDEN_VERSION='v0.5.0'
```

PostgreSQL prompts for passwords when they are not present in the URLs. A temporary [password file](https://www.postgresql.org/docs/current/libpq-pgpass.html) is safer for automation.

## 1. Stop writes

Stop the old Warden instance first. Leave PostgreSQL running.

Stop the new Warden instance too. It must not create or migrate tables while the restore is running.

The exact command depends on the deployment. For Kubernetes, scale the workload to zero:

```bash
kubectl -n warden scale deployment/warden --replicas=0
```

Do not continue until no Warden process is connected to either database.

## 2. Dump the source database

Create a custom-format dump and a checksum:

```bash
pg_dump --format=custom --no-owner --no-privileges \
  --dbname="$SOURCE_DB_URL" \
  --file=warden.dump

shasum -a 256 warden.dump > warden.dump.sha256
pg_restore --list warden.dump > /dev/null
```

Keep both `warden.dump` and `warden.dump.sha256` until the migration is verified.

## 3. Prepare the destination

This deletes everything currently inside the destination database. Check the URL before pressing Enter:

```bash
printf '%s\n' "$DESTINATION_DB_URL"
psql "$DESTINATION_DB_URL" -v ON_ERROR_STOP=1 \
  -c 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;'
```

If the destination already contains data that matters, dump it before this step.

## 4. Restore

Restore the dump and stop immediately if PostgreSQL reports an error:

```bash
shasum -a 256 -c warden.dump.sha256

pg_restore --exit-on-error --no-owner --no-privileges \
  --dbname="$DESTINATION_DB_URL" \
  warden.dump
```

Compare the important row counts before starting Warden:

```bash
for database_url in "$SOURCE_DB_URL" "$DESTINATION_DB_URL"; do
  psql "$database_url" -v ON_ERROR_STOP=1 -c \
    'SELECT (SELECT count(*) FROM monitors) AS monitors,
            (SELECT count(*) FROM groups) AS groups,
            (SELECT count(*) FROM monitor_checks) AS checks,
            (SELECT count(*) FROM users) AS users;'
done
```

The two results must match. A small difference means the old Warden was not fully stopped before the dump.

## 5. Start the new Warden

Point the new deployment's `DB_URL` at `DESTINATION_DB_URL` and initially run the same `WARDEN_VERSION` as the source. This separates the database move from an application upgrade and makes rollback predictable.

Start Warden. For Kubernetes:

```bash
kubectl -n warden scale deployment/warden --replicas=1
kubectl -n warden rollout status deployment/warden
```

Warden runs any pending schema migrations during startup. Verify the application before changing DNS or shutting down the old database:

```bash
curl --fail --silent --show-error "$WARDEN_URL/healthz"
```

Then sign in and confirm that monitors, groups, incidents, users, settings, and status pages are present.

## 6. Relearn latency from the new location

This step is required when the new Warden instance has a materially different network path, such as moving from a cloud region to a residential homelab.

In Warden, open **Settings → Notification Intelligence → High Latency**, click **Relearn from now**, and confirm.

That action:

- keeps all check and uptime history;
- forgets the derived p50 and p95 baselines;
- closes current degraded-latency alerts without sending a recovery storm;
- ignores old latency samples while learning;
- uses the fixed global latency threshold until each monitor has enough new successful checks.

No Warden restart is required. Do not lower the global threshold during the learning period.

## 7. Cut over and clean up

Only after the checks above pass:

1. point the public URL or DNS at the new Warden instance;
2. watch real checks and notification delivery;
3. keep the old database and dump during your rollback window;
4. upgrade Warden separately after the migration is stable.

## Roll back

Stop the new Warden instance, point the public URL back to the old instance, and start the old Warden with its original database. Because the old database was left untouched, rollback does not require restoring the dump.

Do not run both instances against different databases under the same public URL: they will check and notify independently.
