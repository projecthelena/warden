# Notification Fatigue Prevention

Warden prevents alert fatigue with three mechanisms that work together. A notification only fires when a problem is **confirmed, not flapping, and not in cooldown**.

## How It Works

### Confirmation Checks

A single failed check doesn't trigger an alert — it could be a transient blip. Warden waits for **N consecutive failures** before confirming a monitor is down and sending a notification. Same logic applies to degraded (high latency) checks.

```
Check 1: DOWN  → count=1/3 → no alert
Check 2: DOWN  → count=2/3 → no alert
Check 3: DOWN  → count=3/3 → CONFIRMED → alert sent
Check 4: UP    → RECOVERED → recovery alert sent, counter reset
```

Default: **3 consecutive failures**. Set to 1 for immediate alerts.

### Notification Cooldown

After an alert fires, repeat notifications for the same event type are suppressed for a cooldown period. This prevents getting an alert every check interval while a monitor stays down.

Recovery notifications ("Monitor Recovered") bypass cooldown — you always want to know when things come back.

Default: **30 minutes**. Set to 0 to disable.

### Flap Detection

If a monitor rapidly oscillates between UP and DOWN, Warden detects it as "flapping" and suppresses all notifications until the monitor stabilizes. You get a single "flapping" alert when it starts and a "stabilized" alert when it stops.

It works by measuring the percentage of state transitions in a sliding window. Uses hysteresis (start threshold: 25%, stop threshold: 20%) so the detection itself doesn't oscillate.

Default: **enabled**, 25% threshold over last 21 checks.

## Configuration

All settings live in **Settings** on the dashboard. Changes apply immediately to all running monitors.

| Setting | Default | Range |
|---------|---------|-------|
| Confirmation threshold | 3 | 1-100 |
| Cooldown minutes | 30 | 0-1440 |
| Flap detection enabled | true | true/false |
| Flap window (checks) | 21 | 3-100 |
| Flap threshold (%) | 25 | 1-100 |

### Per-Monitor Overrides

**Confirmation threshold** and **cooldown** can be overridden on individual monitors (in the monitor's Advanced Settings). This lets you set threshold=1 on critical monitors while keeping threshold=5 on less important ones. When not set, the global default is used.

Flap detection settings are global only.
