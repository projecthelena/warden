# Notification Fatigue Prevention

Warden prevents alert fatigue with several mechanisms that work together. A notification only fires when a problem is **confirmed, sustained, not already covered by a bigger incident, not from a monitor that has been shouting all day, not flapping, and not in cooldown**.

## How It Works

### Sustained Outages

Confirming a monitor is down and telling you about it are two different moments. When a monitor is confirmed down Warden opens an outage and says nothing; the outage is announced only if it is **still** down after the sustained window. A blip that resolves inside that window is recorded in the history and the daily digest, and never interrupts anyone.

```
00:00  check fails, threshold met  → outage opens, silent
00:02  still down                  → still silent
00:03  still down                  → ALERT: "Monitor is down (Status: 503) — down for 3m"
00:33  still down                  → reminder: "Still down after 33m"
01:33  still down                  → reminder: "Still down after 1h33m"
01:40  recovers                    → recovery sent, because the outage had been announced
```

The same ladder applies to degraded (high latency) outages.

A recovery is announced **only if the outage itself was**. Under the default policy most short outages are never announced, so telling you they recovered would be pure noise.

Default: announce after **180 seconds**, first reminder after **30 minutes**, then every **60 minutes**. Set the sustained window to 0 to alert the moment an outage opens, or the reminder interval to 0 to turn reminders off.

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

After a flapping or stabilized alert fires, repeats of the same event type are suppressed for a cooldown period.

Down and degraded no longer use the cooldown: how often an ongoing outage repeats is governed by the reminder interval in the sustained ladder above, which knows how long the outage has actually lasted. The cooldown only ever knew how long ago the last message was.

Default: **30 minutes**. Set to 0 to disable.

### Flap Detection

If a monitor rapidly oscillates between UP and DOWN, Warden detects it as "flapping" and suppresses all notifications until the monitor stabilizes. You get a single "flapping" alert when it starts and a "stabilized" alert when it stops.

It works by measuring the percentage of state transitions in a sliding window. Uses hysteresis (start threshold: 25%, stop threshold: 20%) so the detection itself doesn't oscillate.

Default: **enabled**, 25% threshold over last 21 checks.

### Correlated Incidents

Monitors that fail together are usually one thing failing. When enough of a group opens an outage inside the correlation window, Warden sends **one** message naming the group and the affected monitors instead of one per monitor.

"Enough" is a percentage of the group with an absolute floor, so it still means something whether the group has 12 monitors or 200: `max(3, 30% of the group)`.

```
19:29  eleven of twelve NodeSource monitors start failing, all with 404
19:32  ALERT: "11 of 12 monitors in NodeSource are down, for 3m: ..."
20:02  reminder: one message, not eleven
```

Reminders follow the incident, not the monitor: outages announced together share a correlation id and are reminded about together.

### Probe-Wide Failures

Warden watches from one place. If **80% of every monitor across at least two groups** goes down at once, the likely explanation is Warden's own network rather than every unrelated service failing simultaneously — so it says that, once, instead of blaming each target.

The two-group requirement matters: a single group going down entirely looks identical from here, and "your NodeSource group is down" is both more specific and safer to be wrong about than "your network is broken".

### Repeat-Offender Damping

A monitor that has already interrupted you three times in 24 hours is describing itself, not an event. On the third alert Warden says so explicitly and then stops sending individual alerts for it:

```
Dogfood SaaS has alerted 3 times in the last 24h and is down again.
Muting its individual alerts until it settles — it stays in the daily
digest and on the dashboard.
```

Subsequent outages are still recorded, still shown, still in the digest. They just do not interrupt. Because those outages are never marked as announced, their recoveries stay quiet too. Set the limit to 0 to turn the damping off.

### Per-Monitor Mute

Any monitor can have its alerts muted from its details panel (**Alerts → Mute Alerts**). A muted monitor is still checked, still records outages, and still appears in the digest and on the dashboard — it simply never interrupts anyone. This is the right tool for test and staging targets that should not wake anyone.

## Configuration

All settings live in **Settings** on the dashboard. Changes apply immediately to all running monitors.

| Setting | Default | Range |
|---------|---------|-------|
| Confirmation threshold | 3 | 1-100 |
| Announce after (seconds) | 180 | 0-86400 |
| First reminder (minutes) | 30 | 0-10080 |
| Repeat reminder (minutes) | 60 | 0-10080 |
| Cooldown minutes | 30 | 0-1440 |
| Correlation window (seconds) | 300 | 0-86400 |
| Correlation minimum monitors | 3 | 1-1000 |
| Correlation group share (%) | 30 | 1-100 |
| Probe-wide share (%) | 80 | 1-100 |
| Repeat-offender limit | 3 | 0-1000 |
| Repeat-offender window (minutes) | 1440 | 1-43200 |
| Flap detection enabled | true | true/false |
| Flap window (checks) | 21 | 3-100 |
| Flap threshold (%) | 25 | 1-100 |

### Per-Monitor Overrides

**Confirmation threshold** and **cooldown** can be overridden on individual monitors (in the monitor's Advanced Settings). This lets you set threshold=1 on critical monitors while keeping threshold=5 on less important ones. When not set, the global default is used.

Flap detection settings are global only.

## The digest does not silence anything

Selecting an event under **Daily Digest → Include in the digest** controls what the daily summary covers. It used to *divert* the event, so choosing "Down" there silenced outage alerts entirely — an easy way to end up with no notifications at all without realising it.

Those are now two independent decisions:

- **What the digest covers** — Daily Digest → Include in the digest.
- **What reaches you immediately** — the per-event toggles under Event Types.

An event can be in both. To stop an immediate alert, turn the event off; putting it in the digest no longer does that.
