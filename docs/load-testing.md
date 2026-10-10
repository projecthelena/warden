# Load Testing

**Validated SQLite capacity: 1,000 HTTP monitors at a 10-second interval (~100 checks/s), sustained for 18 hours on Node A with the configuration below.** Use this as the current tested sizing target. The 2,000-monitor stage is not approved. This is a measured operating envelope for the stated hardware, release, workload, and duration; it is not a guarantee for arbitrary hardware, longer history, or slower targets.

## Configuration and scope

The approved run used Warden `v0.10.0-rc.1`, revision `3ff86808c04af8bdfce9d88c84bce3584e0c13fa`, pinned to image digest `sha256:ea3a3f94b96e33f53600a7776a384a1dcce42fbddd9e3501d2c7b69aab8c87af`.

| Setting | Value |
| --- | --- |
| Database | SQLite, new empty database for each run |
| Monitors | 1,000 HTTP monitors, every 10 seconds |
| Target | Controlled healthy HTTP endpoint; automatic HTTP diagnostics enabled |
| SQLite rollup batch size | `SQLITE_ROLLUP_BATCH_SIZE=10` (the default is 50) |
| Rollup instrumentation | `ROLLUP_DIAGNOSTICS_ENABLED=true` |
| Warden container | 100m CPU / 128 MiB memory requests; no CPU limit; 2 GiB memory limit |
| Readers | k6 1.3.0; peak 5 public + 1 operator journeys/s |
| Reader ramp / hold / cooldown | 5 / 23 / 2 minutes |
| Total duration | 18 hours after seeding completes |

Node A: Intel i5-12600K (10 cores / 16 threads), 32 GB RAM, Samsung 980 1 TB NVMe SSD. Node B: Intel i3-8100 (4 cores / 4 threads), 32 GB RAM, P3 1 TB SATA SSD. Both run shared k3s workloads and have 1 GbE networking. These are benchmark machines, not minimum hardware requirements.

The generator, supervisor, and target run on Node A in both cases. Warden runs on Node A for the first case and Node B for the repeat. Consequently, the first case has a colocated target and readers, while the repeat crosses the LAN. Whole-host measurements include unrelated applications and generator overhead; they are not application-only resource requirements or an isolated CPU-hardware comparison.

## Results

Node A ran from **2026-10-09 09:55:42 UTC to 2026-10-10 03:55:50 UTC**. Its supervisor recorded `passed`. All checks completed in that window were up, with zero scheduling drops, persistence errors, readiness 503s, rollup errors, or process restarts. The mixed reader phase passed 53,264 assertions with zero HTTP failures or dropped iterations. The supervisor then paused all 1,000 monitors without failures and stopped the target.

<!-- results:start -->

| Measurement | Node A | Node B |
| --- | --- | --- |
| Outcome | Passed | Pending |
| Measured duration | 18.002 h | Pending |
| Completed checks in measured window | 6,480,862 | Pending |
| Mean check throughput | 100.00/s | Pending |
| Mean Warden process CPU | 0.197 cores | Pending |
| Warden RSS: median / p95 / max | 66.62 / 67.42 / 69.54 MiB | Pending |
| Whole-host CPU: median / p95 / max | 5.46% / 6.81% / 8.93% | Pending |
| Whole-host memory: median / p95 / max | 36.64% / 41.77% / 42.22% | Pending |
| Mixed HTTP readers | 30,111 requests; p95 39.69 ms; p99 56.11 ms | Pending |
| Counter increases | down: 0, drops: 0, persist_errors: 0, ready_503: 0, rollup_errors: 0 | Pending |

Node B is pending. No capacity or resource comparison is approved for that run until its final summary and integrity checks are available.
<!-- results:end -->

Machine-readable, sanitized aggregates are available in [the result summary](../loadtest/results/sqlite-1000-2026-10.json). Raw time-series exports, LAN addresses, credentials, and host identities are not included.

### Measurement method

- Process CPU is the increase in `process_cpu_seconds_total` divided by elapsed seconds across the measured window. A value of 0.197 cores means roughly 19.7% of one logical CPU on average, not 19.7% of the entire machine. It does not describe peak CPU demand.
- Process memory is `process_resident_memory_bytes` sampled approximately every 10 seconds. RSS does not include the entire filesystem page cache or all container memory accounting.
- Whole-host CPU is cAdvisor's two-minute CPU rate divided by the host's logical CPU count. Whole-host memory is cAdvisor working-set bytes divided by physical memory. Both are sampled approximately every 30 seconds and filtered to the measured window.
- Percentiles use the nearest-rank method over collected samples. Peaks are sampled maxima, not guaranteed instantaneous maxima. The approved run includes 6,464 process samples and 2,159 whole-host samples.
- Node A's readiness latency had a 40.24 ms p95 and a 1,223.53 ms sampled maximum. The final sample's small result queue is not counted as a drop; acceptance also checks loss counters and sustained queue pressure.
- A complete five-minute pre-run idle baseline was not preserved with Node A's approved-run artifacts. Its whole-host figures are therefore absolute usage, not baseline-subtracted Warden overhead. Node B captures a five-minute idle baseline; compare process CPU/RSS directly and treat whole-host differences as contextual.

## Workload and acceptance

The run begins after monitor seeding: 10 minutes of warmup, 30 minutes without mixed readers, 30 minutes of mixed k6 readers, then the remaining 16 hours 50 minutes with monitoring active. The reader phase exercises status overview, monitor pagination, operator overview, search, filters, monitor details, uptime, latency, and recent events using [the load-test tools](../loadtest/README.md).

Acceptance requires at least 99 completed checks/s over five-minute windows after warmup; zero new down checks, scheduling drops, persistence errors, readiness 503s, rollup errors, or restarts; no queue depth above 500 for six consecutive samples; and process RSS below 2 GiB. The host guard stops the run above 70% CPU or 75% working-set memory, or after repeated missing host metrics. k6 must pass its configured latency/error thresholds with zero dropped iterations. The controlled target is probed separately; a target or supervisor failure must not be reported as proven database-capacity failure.

This report records the 18-hour diagnostic acceptance run, which is distinct from the incremental four-case protocol in the tool README. A full 24-hour contention investigation, repeated same-host runs, a diagnostics-disabled control, browser interaction verification, and the complete SQLite/PostgreSQL matrix are not established by this result. This run does not cover arbitrary monitor types, slow/failing Internet endpoints, or steady-state performance after weeks of accumulated history.

## Unapproved stage: 2,000 monitors

The same release with SQLite batch size 10 on Node B stopped on 2026-10-09 at 13:59:02 UTC, approximately four hours after starting, with `TimeoutError`. The last successful sample recorded 2,909,891 cumulative checks and approximately 200 checks/s; subsequent logs showed readiness 503 responses. The run did not satisfy acceptance, even though its mixed-reader phase passed. Rollup and database connection contention were observed around the failure, but this does not establish exclusive causality.

Keep the current SQLite sizing target at **1,000 monitors / 10 seconds under the tested configuration**. A passing 1,000-monitor repeat can extend the evidence to Node B; it cannot validate the failed 2,000-monitor stage or establish a universal maximum.
