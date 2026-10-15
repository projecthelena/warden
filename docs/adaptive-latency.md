# Adaptive latency baselines

Every monitor learns its own normal latency from successful checks. High-latency warnings use a configurable percentage above the monitor’s **p95**: the response time at or below which approximately 95% of successful checks fall.

## Configure warnings

In **Settings → Alerting → High latency**, set **Above normal (%)**. The default is **50%**. An individual monitor can override this percentage under **Alerting**; leave it empty to inherit the global percentage.

```
threshold = ceil(p95 × (1 + percentage / 100))
p95 = 1000ms, percentage = 25 → degraded above 1250ms
p95 =  100ms, percentage = 25 → degraded above  125ms
```

A check exactly at the threshold is not degraded. Rounding is upward to the next whole millisecond. There is no fixed millisecond fallback or additional 100ms margin. Fast targets may need a larger percentage to tolerate ordinary jitter.

Crossing the threshold uses the existing confirmation, cooldown, sustained-alert, recovery and flapping rules. It does not immediately send a notification on every slow check.

## Learning

The baseline uses the last **7 days** by default and requires **200 successful checks**. Warden recomputes it hourly and persists it across restarts. At a 60-second interval, initial learning takes at least 3 hours 20 minutes, plus up to an hour until the next baseline refresh.

Until a usable baseline exists, latency warnings are inactive. Failed checks still trigger normal down detection and notifications. Failed requests never contribute their latency to the baseline.

Changing the target keeps the monitor’s history; the baseline adapts as that history rolls out of the window. After moving Warden to another network, use **Relearn from now** to clear derived baselines and start learning from new checks. This closes current degraded outages while preserving check history. Latency warnings resume after enough new successful checks.

## Upgrading from fixed thresholds

Old global and per-monitor millisecond limits no longer govern latency warnings. Existing monitors inherit the global percentage until you set their own percentage. Stored fixed values remain available for database rollback and are never interpreted as percentages.

An explicitly stored multiplier such as 150% of p95 becomes 50% above p95. Otherwise the new default is 50%. The former adaptive enable switch and absolute margin no longer apply.

API clients should send `latencyIncreasePercent` on monitor create/update and `notification.latency.increase_percent` in settings. Values must be integers from 1 to 9900. Omit the monitor override to inherit the global percentage. Requests using the retired latency settings are rejected with HTTP 400.

## Inspecting a baseline

The MCP `get_monitor_latency` tool reports `baselineP50Ms`, `baselineP95Ms`, `baselineSamples` and the running monitor’s `degradedAboveMs`. No effective threshold is reported while learning or when the monitor is not running.

See [Notification Fatigue Prevention](notification-fatigue.md) for alert confirmation, reminders, correlation and flapping detection.
