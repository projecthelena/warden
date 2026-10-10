# Warden load testing

This directory contains the tools used to run the same test against four disposable Warden installations:

| Case | CPU | Memory | Storage | Network | Database |
| --- | --- | --- | --- | --- | --- |
| `node-a-sqlite` | Intel i5-12600K, 10C/16T | 32 GB | Samsung 980 1 TB NVMe SSD | 1 GbE | SQLite |
| `node-a-postgres` | Intel i5-12600K, 10C/16T | 32 GB | Samsung 980 1 TB NVMe SSD | 1 GbE | PostgreSQL |
| `node-b-sqlite` | Intel i3-8100, 4C/4T | 32 GB | P3 1 TB SATA SSD | 1 GbE | SQLite |
| `node-b-postgres` | Intel i3-8100, 4C/4T | 32 GB | P3 1 TB SATA SSD | 1 GbE | PostgreSQL |

The [load-testing report](../docs/load-testing.md) contains the measured SQLite capacity, resource usage, and scope of the conclusions. This document records the procedure; the report identifies which stages have actually been completed.

## Procedure

1. Record a five-minute idle baseline for both nodes before each case. These are shared homelab hosts: Node A also runs the k3s control plane and mixed application/data workloads, while Node B runs the k3s agent and media workloads. The final report records the actual baseline CPU, memory, disk, network, and active workloads for every run.

2. Deploy one disposable Warden case using the homelab runbook. Do not run these tests against the production instance.

3. On the other node, start the controlled HTTP target without request logging:

   ```bash
   go run ./cmd/faketarget -listen :8888 -quiet
   ```

4. Initialize Warden and create the status page used by the reader test.

5. Create the first 100 monitors, pointing them at the controlled target's LAN address:

   ```bash
   WARDEN_URL=https://warden-under-test.example \
   WARDEN_USERNAME=admin \
   WARDEN_PASSWORD='...' \
   TARGET_URL=http://load-generator.lan:8888/healthy \
   go run ./cmd/stress_test -count 100 -interval 10
   ```

6. Wait five minutes before measuring so the process, database connections, and monitor schedule are warm.

7. Start the mixed reader workload from the node running the controlled target. It models two independent journeys:

   - A public visitor loads the compact status overview and expands the first monitor page for the load-test group.
   - An authenticated operator loads the dashboard overview, paginates the group, searches, cycles through Operational/Issues/Paused filters, and opens a monitor. Opening the monitor loads uptime, one-hour latency, and recent events in parallel, like the application.

   The rate variables are journeys per second, not raw HTTP requests per second. A public journey makes two requests and an operator journey makes eight. Keep the operator rate much lower than the public rate; concurrent administrators are naturally rarer than status-page readers.

   ```bash
   mkdir -p loadtest/results

   WARDEN_URL=https://warden-under-test.example \
   STATUS_SLUG=public-page \
   GROUP_ID=g-load-test-example \
   WARDEN_USERNAME=admin \
   WARDEN_PASSWORD='...' \
   PUBLIC_PEAK_RPS=5 \
   OPERATOR_PEAK_RPS=1 \
   RESULT_FILE=loadtest/results/node-a-sqlite-100-run1.json \
   k6 run loadtest/api.js
   ```

   `GROUP_ID` is recommended so an empty default group cannot be selected accidentally. `PAGE_SIZE` defaults to the UI default of 25, and `SEARCH_TERM` defaults to `Load Monitor 00500`.

   Use `TEST_PROFILE=public` or `TEST_PROFILE=operator` to isolate a limit after the mixed smoke test. The operator profile requires credentials; the public profile does not. Never commit credentials or place them in a result filename.

8. Leave the workload at its configured peak for the full test duration. VictoriaMetrics records Warden and k3s metrics independently. k6 writes one aggregated JSON summary when the run finishes. The run fails if more than 1% of requests or checks fail, if any iteration is dropped, or if journey latency exceeds the configured p95/p99 thresholds.

9. While the mixed workload is at its peak, use a browser to log in, open the 1,000-monitor group, paginate, search, apply every status filter, and open several monitor pages. k6 verifies the same data requests and their response contracts, while this short browser pass verifies React rendering and interaction remain usable under load. Record any visible timeout, stale state, broken navigation, or UI lock-up in the run notes.

10. Repeat the measured run once without changing the deployment or parameters. Save it as `run2`.

11. Add 150, 250, and 500 monitors to the same group in successive stages, producing totals of 250, 500, and 1,000. Pass the existing `-group-id` and set `-start-index` to the current monitor count so names remain unique; for example, use `-group-id g-load-test-example -start-index 100 -count 150` for the second batch. Repeat the warm-up and measured runs after each addition and include the total monitor count in each result filename.

12. Remove the disposable Warden release, deploy the next case, and repeat from step 1. Change only the node and database between cases.

13. Build the final report from the aggregated summaries and their matching VictoriaMetrics time windows. Do not commit credentials or raw time-series exports.

Warden limits a single client IP to 100 HTTP requests per second with a burst of 200. Account for the requests per journey when increasing rates. Tests above that limit require multiple load-generator addresses and must be reported separately from this four-case comparison.

## Diagnosing rollup contention

Use a disposable instance with `ROLLUP_DIAGNOSTICS_ENABLED=true` and a private `OBSERVABILITY_ADDR` listener. Record the exact image revision, flag state, database, retention, monitor count and interval for every run. Keep the controlled target supervised and verify its health independently throughout the soak; a failed target invalidates the capacity measurement.

1. Establish a clean 1,000-monitor / 10-second SQLite case and a clean 2,000-monitor / 10-second PostgreSQL case on the same node. Do not reuse a failed database or increase to 3,000 monitors yet. Warm up for ten minutes, capture thirty minutes without reader traffic, then run the same mixed public/operator workload for thirty minutes and soak for at least four hours. When reproducing a failure that first appeared after several hours of history growth, extend the run to at least 24 hours; a clean short run does not rule it out.
2. Correlate readiness 503 timestamps, scheduling drops, persist duration, SQL pool waits and queue depth with the rollup phase histograms and slow-operation logs. Preserve results and relevant time windows before metric retention expires. Queue depth sampled every few seconds can miss short saturation bursts; zero sampled depth alone is not proof of zero loss.
3. If connection acquisition dominates, identify what else held the pool. If query/scan dominates, inspect the aggregation plan and history scanned. If begin/upsert/commit dominates, investigate transaction size, locking and storage latency. Compare PostgreSQL before selecting an optimization; retain the same workload and history window for a controlled before/after comparison.
4. Accept a stage only with zero dropped schedules, no new persistence errors, no readiness failures or restarts, expected completed-check throughput (100/s at 1,000 monitors, 200/s at 2,000), and no unexplained UI history gaps. Stop escalation at the first failure. Preserve the existing readiness timeout and queue sizes while diagnosing.
5. After a measured fix, repeat the failed SQLite stages and the PostgreSQL control, then complete the four-case matrix. Repeat a matched run with diagnostics disabled to assess instrumentation overhead before publishing capacity results.

This instrumentation does not fix scheduling loss or change the storage algorithm. A scheduler fix should bound each monitor to one pending/in-flight check and make coalescing explicit; increasing the queue alone only postpones saturation. SQLite query/transaction changes require evidence from the diagnostic phases and regression coverage for uptime history.
