# Load-Test Results

**Use PostgreSQL above 1,000 monitors.** With PostgreSQL, Warden sustained 5,000 HTTP monitors at a 10-second interval (~500 checks/s) for 18 hours with continuous readers, on a local NVMe disk. With SQLite, one machine sustained 1,000 monitors for 18 hours, a repeat on a second machine failed, and 2,000 monitors did not pass.

Both results describe one release, one workload, and specific hardware. They are not capacity guarantees or minimum hardware requirements. Validate your own deployment before relying on these numbers.

| Database | Monitors / interval | Release | Result |
| --- | --- | --- | --- |
| PostgreSQL 18.6 | 5,000 / 10 s | `v0.10.0-rc.6` | Passed 18 h on Node A |
| SQLite | 1,000 / 10 s | `v0.10.0-rc.1` | Passed 18 h on Node A; failed after 6 h 55 min on Node B |
| SQLite | 2,000 / 10 s | `v0.10.0-rc.1` | Failed after about 4 h on Node B |

Node A: Intel i5-12600K (10 cores / 16 threads), 32 GB RAM, Samsung 980 1 TB NVMe SSD. Node B: Intel i3-8100 (4 cores / 4 threads), 32 GB RAM, P3 1 TB SATA SSD. Both run shared k3s workloads and have 1 GbE networking.

## PostgreSQL: 5,000 monitors for 18 hours

The run used Warden `v0.10.0-rc.6`, revision `6a082716a9322ee0f5aa9b02d8da348e9e7f2096`, pinned to image digest `sha256:0c8e387a205370bc6645c71de260ea714b4a20a83ead05b651b3878f9685ba93`. It ran from **2026-10-25 10:08:09 UTC to 2026-10-26 04:08:17 UTC**, and the supervisor recorded `passed`.

| Setting | Value |
| --- | --- |
| Database | PostgreSQL 18.6 on Node A's NVMe disk, `data_checksums` on |
| PostgreSQL settings | `shared_buffers=1GB`, `max_wal_size=1GB`, `wal_compression=off`, `checkpoint_timeout=5min`, `synchronous_commit=on` (Warden turns it off per check batch) |
| Starting data | A copy of an earlier load-test database with existing check history; 44 GB at the end of the run |
| Monitors | 5,000 HTTP monitors, every 10 seconds |
| Target | Controlled healthy HTTP endpoint; automatic HTTP diagnostics enabled |
| Uptime counters | `INCREMENTAL_UPTIME_ENABLED=true` |
| Warden container | 100m CPU / 128 MiB memory requests; no CPU limit; 2 GiB memory limit |
| PostgreSQL container | 100m CPU / 256 MiB memory requests; no CPU limit; 4 GiB memory limit |
| Readers | k6 1.3.0, from minute 40 to the end in 18 consecutive one-hour chunks, each with a new login and its own thresholds |
| Total duration | 18 hours after seeding completes |

<!-- postgres-results:start -->

| Measurement | Node A |
| --- | --- |
| Outcome | Passed |
| Measured duration | 18.002 h |
| Completed checks in measured window | 32,404,245 |
| Mean check throughput | 500.00/s (lowest five-minute window 499.92/s) |
| Mean Warden process CPU | 0.584 cores |
| Warden RSS: median / p95 / max | 175.82 / 179.05 / 182.43 MiB |
| Queue depth max (capacity 1,000 each) | checks: 0, results: 125 |
| Readiness: median / p95 / max | 1.43 / 4.27 / 31.74 ms |
| Mixed HTTP readers | 1,118,688 requests, 0 errors; p95 40.81–42.01 ms and p99 49.15–52.45 ms per chunk; max 1,666 ms |
| Counter increases | down: 0, drops: 0, persist_errors: 0, ready_503: 0 |
| Write-ahead log | 56.8 GB in total, at most 125 MB/min |
| Checkpoints | 216 timed, 0 requested by WAL volume |

<!-- postgres-results:end -->

The run covered 18 hourly latency-baseline refreshes. Before `v0.10.0-rc.6`, each refresh could write gigabytes of WAL in a few minutes and force checkpoints; migration 00038 and the lower write cost per check in 00039 removed that burst. In an earlier 90-minute run with the same release, one refresh added about 50 MB of WAL.

Machine-readable, sanitized aggregates are in [the PostgreSQL result summary](../loadtest/results/postgresql-5000-2026-10.json). Process CPU, memory, and readiness are measured as described for the SQLite runs below.

### Not established

- 10,000 monitors. CPU, memory, and queue depth leave headroom, but WAL volume, table growth, and sensitivity to disk stalls double, and none of it has been measured.
- Resilience to disk stalls with this release. The disk did not stall during the run (see below).
- Other disks, remote or managed PostgreSQL, other monitor types, slow or failing endpoints, and weeks of accumulated history.

## Disk stalls

Every failed PostgreSQL run at 5,000 monitors ended the same way: the disk under PostgreSQL stopped completing I/O for several seconds, a check batch could not commit, both in-memory queues filled, and Warden dropped checks.

Warden holds up to 1,000 scheduled checks and 1,000 results in memory. At 500 checks/s that absorbs about two seconds of a stalled database; at 1,000 checks/s, about one. A longer stall drops checks: they are counted in `warden_monitor_check_scheduling_total{outcome="dropped"}`, never recorded as down, and the affected monitors have a gap in their history.

What we observed:

- **Node B, SATA SSD.** Three runs with `v0.10.0-rc.4` and `v0.10.0-rc.5` failed between 56 minutes and 3 h 41 min. In the traced run the disk completed no I/O for about 7 seconds with requests in flight; a single WAL `fsync` took 8.0 s, the commit of a 50-check batch waited 8.04 s, and Warden dropped 2,027 checks. Each stall coincided with a checkpoint forced by WAL volume; in the traced run, the burst of WAL came from the hourly baseline refresh. The same disk also froze for 1–2 seconds with no load.
- **Node A, NVMe SSD.** The disk paused for about 5 seconds twice with PostgreSQL idle, which failed two 90-minute runs before `v0.10.0-rc.6`. No pause occurred during the 18-hour run.

`v0.10.0-rc.6` removes most of the WAL that triggered these stalls and commits check batches without waiting for the WAL flush, but reads and writes of data files still wait on the disk. It has not been tested against a stall.

For large installations:

- Put PostgreSQL on an NVMe disk, or an SSD with DRAM cache and power-loss protection. Measure `fsync` latency before trusting a disk (for example with `fio --fsync=1`); a disk that is fast on average can still stall for seconds.
- Avoid sharing that disk with other write-heavy workloads.
- Apply the [settings for large installations](database.md#settings-for-large-installations).
- Alert on dropped checks and on `warden_monitor_result_queue_depth` approaching 1,000.

## SQLite: 1,000 monitors (preliminary)

**Preliminary result: one tested configuration sustained 1,000 HTTP monitors at a 10-second interval (~100 checks/s) for 18 hours with SQLite.** A repeat on a second machine failed after 6 hours 55 minutes with a readiness 503 response. These results establish a successful run on Node A, not a general 1,000-monitor capacity guarantee or minimum hardware specification. The 2,000-monitor stage did not pass.

This section covers tests completed on October 9–10, 2026, using `v0.10.0-rc.1`. It is a snapshot of that release and workload, not a benchmark of later releases.

### Configuration and scope

Both 1,000-monitor runs used Warden `v0.10.0-rc.1`, revision `3ff86808c04af8bdfce9d88c84bce3584e0c13fa`, pinned to image digest `sha256:ea3a3f94b96e33f53600a7776a384a1dcce42fbddd9e3501d2c7b69aab8c87af`.

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

### Results

Node A ran from **2026-10-09 09:55:42 UTC to 2026-10-10 03:55:50 UTC**. Its supervisor recorded `passed`. All checks completed in that window were up, with zero scheduling drops, persistence errors, readiness 503s, rollup errors, or process restarts. The mixed reader phase passed 53,264 assertions with zero HTTP failures or dropped iterations. The supervisor then paused all 1,000 monitors without failures and stopped the target.

<!-- results:start -->

| Measurement | Node A | Node B |
| --- | --- | --- |
| Outcome | Passed | Failed |
| Measured duration | 18.002 h | 6.924 h |
| Completed checks in measured window | 6,480,862 | 2,492,766 |
| Mean check throughput | 100.00/s | 100.00/s |
| Mean Warden process CPU | 0.197 cores | 0.209 cores |
| Warden RSS: median / p95 / max | 66.62 / 67.42 / 69.54 MiB | 59.91 / 60.59 / 63.36 MiB |
| Whole-host CPU: median / p95 / max | 5.46% / 6.81% / 8.93% | 7.69% / 12.53% / 14.72% |
| Whole-host memory: median / p95 / max | 36.64% / 41.77% / 42.22% | 19.67% / 21.82% / 21.93% |
| Mixed HTTP readers | 30,111 requests; p95 39.69 ms; p99 56.11 ms | 30,113 requests; p95 40.87 ms; p99 53.31 ms |
| Counter increases | down: 0, drops: 0, persist_errors: 0, ready_503: 0, rollup_errors: 0 | down: 0, drops: 0, persist_errors: 0, ready_503: 1, rollup_errors: 0 |

Node B ran from **2026-10-10 10:18:45 UTC to 2026-10-10 17:14:13 UTC**, with outcome **failed**. Stop reason: `ready_503 increased`. Mean process CPU was 1.06× Node A; peak RSS was 0.91× Node A. These ratios describe these runs, including their different target placement and background workloads. The shorter failed run is not an equal-duration capacity comparison.

<!-- results:end -->

Node B recorded no down checks, scheduling drops, persistence errors, rollup errors, or process restarts during the measured window, but its readiness failure means it did not pass acceptance. Its mixed-reader phase had already completed successfully. The supervisor subsequently paused all 1,000 monitors and stopped the target. The failure does not by itself establish a hardware or database capacity limit; its root cause remains under investigation.

Machine-readable, sanitized aggregates are available in [the result summary](../loadtest/results/sqlite-1000-2026-10.json). Raw time-series exports, LAN addresses, credentials, and host identities are not included.

#### Measurement method

- Process CPU is the increase in `process_cpu_seconds_total` divided by elapsed seconds across the measured window. A value of 0.197 cores means roughly 19.7% of one logical CPU on average, not 19.7% of the entire machine. It does not describe peak CPU demand.
- Process memory is `process_resident_memory_bytes` sampled approximately every 10 seconds. RSS does not include the entire filesystem page cache or all container memory accounting.
- Whole-host CPU is cAdvisor's two-minute CPU rate divided by the host's logical CPU count. Whole-host memory is cAdvisor working-set bytes divided by physical memory. Both are sampled approximately every 30 seconds and filtered to the measured window.
- Percentiles use the nearest-rank method over collected samples. Peaks are sampled maxima, not guaranteed instantaneous maxima. Node A includes 6,464 process samples and 2,159 whole-host samples.
- Node A's readiness latency had a 40.24 ms p95 and a 1,223.53 ms sampled maximum. The final sample's small result queue is not counted as a drop; acceptance also checks loss counters and sustained queue pressure.
- A complete five-minute pre-run idle baseline was not preserved with Node A's approved-run artifacts. Its whole-host figures are therefore absolute usage, not baseline-subtracted Warden overhead. Node B captured a five-minute idle baseline; compare process CPU/RSS directly and treat whole-host differences as contextual.

### Workload and acceptance

The run begins after monitor seeding: 10 minutes of warmup, 30 minutes without mixed readers, 30 minutes of mixed k6 readers, then the remaining 16 hours 50 minutes with monitoring active. The reader phase exercises status overview, monitor pagination, operator overview, search, filters, monitor details, uptime, latency, and recent events using [the load-test tools](../loadtest/README.md).

Acceptance requires at least 99 completed checks/s over five-minute windows after warmup; zero new down checks, scheduling drops, persistence errors, readiness 503s, rollup errors, or restarts; no queue depth above 500 for six consecutive samples; and process RSS below 2 GiB. The host guard stops the run above 70% CPU or 75% working-set memory, or after repeated missing host metrics. k6 must pass its configured latency/error thresholds with zero dropped iterations. The controlled target is probed separately; a target or supervisor failure must not be reported as proven database-capacity failure.

This report records the 18-hour diagnostic acceptance run, which is distinct from the incremental four-case protocol in the tool README. A full 24-hour contention investigation, repeated same-host runs, a diagnostics-disabled control, browser interaction verification, and the complete SQLite/PostgreSQL matrix are not established by this result. This run does not cover arbitrary monitor types, slow/failing Internet endpoints, or steady-state performance after weeks of accumulated history.

### Unapproved stage: 2,000 monitors

The same release with SQLite batch size 10 on Node B stopped on 2026-10-09 at 13:59:02 UTC, approximately four hours after starting, with `TimeoutError`. The last successful sample recorded 2,909,891 cumulative checks and approximately 200 checks/s; subsequent logs showed readiness 503 responses. The run did not satisfy acceptance, even though its mixed-reader phase passed. Rollup and database connection contention were observed around the failure, but this does not establish exclusive causality.

The strongest supported conclusion is **1,000 monitors / 10 seconds for 18 hours on Node A under the configuration above**. Node B has no passing 18-hour result at either 1,000 or 2,000 monitors. Validate your own deployment before relying on this workload in production; longer history, different hardware, target latency, and other monitor types remain outside this evidence.
