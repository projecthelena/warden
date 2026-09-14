# Adaptive Latency Baselines

Warden learns what normal latency looks like for each monitor. This detects a slow service while it is still returning successful responses, without forcing every target to share one arbitrary millisecond threshold.

A health endpoint that normally responds in 20 ms and a remote storefront that normally responds in 450 ms should not become degraded at the same number. Warden gives each one its own baseline.

## How the baseline is learned

For every active monitor, Warden calculates two percentiles from successful checks:

- **p50** is the typical latency.
- **p95** includes normal slow responses while excluding the most unusual tail.

Failed checks do not contribute. Their latency often measures how long a connection took to time out, not how fast the service is when healthy.

By default, Warden uses the last 7 days, requires at least 200 successful checks, and recomputes the baseline once per hour. Baselines are persisted, so restarting Warden does not discard what it learned.

## When a monitor becomes degraded

The default adaptive threshold is:

```text
max(p95 × 1.5, p95 + 100 ms)
```

The multiplier catches meaningful regressions on slower services. The 100 ms floor keeps small variations on very fast services from becoming noise.

For example:

```text
p50 = 169 ms
p95 = 240 ms
adaptive threshold = max(360 ms, 340 ms) = 360 ms
```

If that monitor remains above 360 ms for the configured confirmation and sustained-alert windows, Warden opens a `degraded` outage and can notify your channels. A degraded monitor is still reachable; this state is separate from `down`.

## Which threshold wins

Warden resolves the threshold in this order:

1. A latency threshold explicitly set on the monitor.
2. The monitor's learned baseline, once it has enough samples.
3. The global fixed latency threshold.

A per-monitor threshold is useful when you have an SLA or already know the maximum acceptable latency. Because it is an explicit decision, adaptive learning never replaces it.

## New monitors and learning time

Until a monitor has enough successful samples, it uses the global fixed threshold. With the default minimum of 200 samples, the approximate learning time is:

| Check interval | Time to 200 samples |
| :------------- | :------------------ |
| 30 seconds     | 1 hour 40 minutes   |
| 60 seconds     | 3 hours 20 minutes  |
| 5 minutes      | 16 hours 40 minutes |

The first baseline is stored on the next hourly computation after enough samples exist.

## Moving Warden to another network or region

Latency measures the whole path from Warden to the target. Moving Warden from a nearby datacenter to a home connection, another country, a VPN, or a different cloud region can change every monitor's latency even when none of the targets changed.

Warden deliberately preserves baselines and check history across restarts and migrations. After a move, the old baseline can therefore produce temporary degraded alerts while the new location accumulates enough successful samples. This is expected: the observations really are slower relative to the location that learned the baseline.

Before moving a production instance:

1. Keep `down` notifications enabled so real failures are still reported.
2. Consider routing `degraded`, `flapping`, and `stabilized` events to the daily digest during the learning period.
3. Allow each monitor to collect the configured minimum number of samples from the new location.
4. Review the learned p50, p95, and degraded threshold with the MCP `get_monitor_latency` tool.
5. Re-enable immediate degraded alerts once the new baselines represent the new path.

Do not raise every fixed threshold merely to hide the transition. That throws away the per-monitor signal and can conceal a real regression later.

## Configuration

Open **Settings → Notifications → High Latency** to enable adaptive learning and adjust the window and multiplier. The complete settings are:

| Setting | Default | Meaning |
| :-- | :-- | :-- |
| Learn what is normal for each monitor | On | Enables adaptive thresholds |
| Learn from | 7 days | History window used for p50 and p95 |
| Minimum samples | 200 | Successful checks required before trusting a baseline |
| Slow at | 150% of p95 | Multiplier applied to p95 |
| Floor above p95 | 100 ms | Minimum distance between p95 and the threshold |

Turning adaptive learning off makes every monitor without an explicit override use the global latency threshold. The minimum-sample and floor values currently use their defaults unless `notification.latency.min_samples` and `notification.latency.floor_ms` are changed through the settings API.

## Inspecting a baseline through MCP

The MCP `get_monitor_latency` result includes:

- `baselineP50Ms`
- `baselineP95Ms`
- `baselineSamples`
- `degradedAboveMs`
- recent latency samples and failures

These fields answer both questions that matter: “what is normal for this target?” and “how slow must it become before Warden calls it degraded?”

See [Notification Fatigue Prevention](notification-fatigue.md) for confirmation windows, reminders, correlation, flapping detection, and choosing which events interrupt you.
