# Warden load testing

This directory contains the tools used to run the same test against four disposable Warden installations:

| Case | CPU | Memory | Storage | Network | Database |
| --- | --- | --- | --- | --- | --- |
| `node-a-sqlite` | Intel i5-12600K, 10C/16T | 32 GB | Samsung 980 1 TB NVMe SSD | 1 GbE | SQLite |
| `node-a-postgres` | Intel i5-12600K, 10C/16T | 32 GB | Samsung 980 1 TB NVMe SSD | 1 GbE | PostgreSQL |
| `node-b-sqlite` | Intel i3-8100, 4C/4T | 32 GB | P3 1 TB SATA SSD | 1 GbE | SQLite |
| `node-b-postgres` | Intel i3-8100, 4C/4T | 32 GB | P3 1 TB SATA SSD | 1 GbE | PostgreSQL |

The final report contains the measurements and conclusions. This document records only the procedure needed to reproduce them.

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
