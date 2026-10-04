# Patterns

Alerting answers "is it broken right now". This answers the slower question you only get to by staring at charts: this one climbs for four hours and then restarts, that one only misbehaves during business hours, these two always fail together.

Warden recomputes findings once a day over the last 14 days and replaces them wholesale, so a pattern that stops happening stops being reported. Pausing a monitor clears its findings: nothing is being measured, so there is nothing to report.

Only successful checks feed the detectors. A failed check's latency is the time spent failing, and a single 10-second timeout would add enough to its hour to manufacture a ramp and a reset out of an outage. A stale finding is worse than none — it sends you looking for something that is no longer there.

They show up in three places: in the **Patterns** tab on a monitor's page, through `list_insights` in the MCP, and — if you turn it on — in a weekly summary on your notification channels.

## What it looks for

### Climbs and resets

Latency rises steadily for hours and then drops straight back to normal. This shape does not establish a restart, an OOM kill, or a connection pool problem. Missing hours break a ramp rather than being treated as continuous measurements.

Warden cannot tell you which. It can tell you the shape is there, how steep it is and how often it repeats, which is the part that otherwise costs you an afternoon:

> Checkout API climbs and resets: 9 ramps in 14 days, rising about 130ms/h from a normal of 254ms to as much as 758ms, then dropping straight back.

The fall matters as much as the climb. A service that climbs and stays up is drift, not a sawtooth, and gets reported as drift instead.

### On a schedule

Whether those resets keep a cadence. This is a genuinely different conclusion from an irregular one: a regular period points at a timer — a cron, a restart policy, a lease expiring — while irregular spacing points at traffic. Only one of those is worth going to look for, so Warden will not claim a schedule it cannot see.

### Time of day

Whether a monitor's trouble piles into one part of the day. Concentrated events suggest a time-of-day association; they do not establish load as the cause. The summary gives the band in UTC and in your own timezone, because nobody reading an alert at midnight wants to do that arithmetic.

### Fails with another monitor

At least three distinct outages must overlap and start within five minutes of each other, and at least 70% of each monitor's downtime must overlap the other's. Degraded periods are excluded. A shared cause is not confirmed: compare failed checks, the monitoring host, and shared network dependencies. Related monitors appear in one expandable section; each relationship is with the selected monitor, not necessarily with every other member.

### Getting slower

Median latency this week against last week. This catches the slow slide that never trips any threshold, because every day looks like the one before it:

> API is 40% slower than it was a week ago: a typical response went from 250ms to 350ms. Compare HTTP timings between these periods to locate the slowdown.

Each calendar week needs at least 72 hourly samples. Missing data cannot turn a few days into a week-over-week comparison.

Improvements are reported too, as their own finding labelled "Getting faster" — knowing a fix worked is worth as much as knowing it broke.

## What it deliberately is not

These are explicit rules, not anomaly detection. A finding you cannot explain is a finding nobody acts on, and with a couple of dozen monitors the rules win on both accuracy and arguability. Every finding carries the numbers behind it so you can disagree with it. Expand **View evidence** for the last analysis time and the number of checks with HTTP traces in the window. Trace coverage and retry recoveries are context, not proof of a root cause. Use **Review checks and HTTP traces** to inspect individual requests.

Findings are labelled `high` or `medium` confidence. Medium means the rule only just matched, and is shown as "worth a look" rather than stated flatly.

## Weekly summary

Off by default — an upgrade should not start sending a new kind of message. Turn it on under **Settings → Weekly Patterns** and pick a day and time; it uses the same timezone as the daily digest.

A week with nothing to report sends nothing. The daily digest already confirms Warden is alive; a weekly "no patterns this week" would just be another thing to learn to ignore.

The marker for "already sent this week" is stored in the database rather than in memory, so a restart neither re-sends nor skips.

## Configuration

| Setting                | Default |
| ---------------------- | ------- |
| Weekly summary enabled | false   |
| Weekly summary day     | Monday  |
| Weekly summary time    | 09:00   |

The detection window (14 days), the daily cadence and the detector thresholds are not configurable. They are chosen to be conservative: the cost of a false pattern is someone spending an afternoon chasing it.
