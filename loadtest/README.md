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

7. Start the reader workload from the node running the controlled target:

   ```bash
   mkdir -p loadtest/results

   WARDEN_URL=https://warden-under-test.example \
   STATUS_SLUG=public-page \
   PEAK_RPS=50 \
   RESULT_FILE=loadtest/results/node-a-sqlite-100-run1.json \
   k6 run loadtest/api.js
   ```

8. Leave the workload at its configured peak for the full test duration. VictoriaMetrics records Warden and k3s metrics independently. k6 writes one aggregated JSON summary when the run finishes.

9. Repeat the measured run once without changing the deployment or parameters. Save it as `run2`.

10. Add 150, 250, and 500 monitors in successive stages, producing totals of 250, 500, and 1,000. Set `-start-index` to the current monitor count so names remain unique; for example, use `-start-index 100 -count 150` for the second batch. Repeat steps 6–9 after each addition and include the total monitor count in each result filename.

11. Remove the disposable Warden release, deploy the next case, and repeat from step 1. Change only the node and database between cases.

12. Build the final report from the aggregated summaries and their matching VictoriaMetrics time windows. Do not commit credentials or raw time-series exports.

Warden limits a single client IP to 100 requests per second with a burst of 200. Tests above that rate require multiple load-generator addresses and must be reported separately from this four-case comparison.
