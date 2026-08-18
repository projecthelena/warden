# Patterns

Alerting answers "is it broken right now". This answers the slower question you only get to by staring at charts: this one climbs for four hours and then restarts, that one only misbehaves during business hours, these two always fail together.

Warden recomputes findings once a day over the last 14 days and replaces them wholesale, so a pattern that stops happening stops being reported. Pausing a monitor clears its findings: nothing is being measured, so there is nothing to report.

Only successful checks feed the detectors. A failed check's latency is the time spent failing, and a single 10-second timeout would add enough to its hour to manufacture a ramp and a reset out of an outage. A stale finding is worse than none — it sends you looking for something that is no longer there.

They show up in three places: on a monitor's page under **Patterns**, through `list_insights` in the MCP, and — if you turn it on — in a weekly summary on your notification channels.

## What it looks for

### Climbs and resets

Latency rises steadily for hours and then drops straight back to normal. The signature of something being recycled: a restart, an OOM kill, a connection pool being rebuilt.

Warden cannot tell you which. It can tell you the shape is there, how steep it is and how often it repeats, which is the part that otherwise costs you an afternoon:

> homedepot-nucleus-prod-3 climbs and resets: 9 ramps in 14 days, rising about 130ms/h from a normal of 254ms to as much as 758ms, then dropping straight back.

The fall matters as much as the climb. A service that climbs and stays up is drift, not a sawtooth, and gets reported as drift instead.

### On a schedule

Whether those resets keep a cadence. This is a genuinely different conclusion from an irregular one: a regular period points at a timer — a cron, a restart policy, a lease expiring — while irregular spacing points at traffic. Only one of those is worth going to look for, so Warden will not claim a schedule it cannot see.

### Time of day

Whether a monitor's trouble piles into one part of the day. Events spread evenly are chance; 78% of them inside an eight-hour band is load. The summary gives the band in UTC and in your own timezone, because nobody reading an alert at midnight wants to do that arithmetic.

### Fails with another monitor

How much of one monitor's downtime overlaps another's. Two monitors that keep failing together share a cause and should be looked at as one thing.

### Getting slower

Median latency this week against last week. This catches the slow slide that never trips any threshold, because every day looks like the one before it:

> API is 40% slower than it was a week ago: a typical response went from 250ms to 350ms. Nothing alerted, because no single check was slow enough to.

Improvements are reported too, as their own finding labelled "Getting faster" — knowing a fix worked is worth as much as knowing it broke.

## What it deliberately is not

These are explicit rules, not anomaly detection. A finding you cannot explain is a finding nobody acts on, and with a couple of dozen monitors the rules win on both accuracy and arguability. Every finding carries the numbers behind it so you can disagree with it.

Findings are labelled `high` or `medium` confidence. Medium means the rule only just matched, and is shown as "worth a look" rather than stated flatly.

## Weekly summary

Off by default — an upgrade should not start sending a new kind of message. Turn it on under **Settings → Weekly Patterns** and pick a day and time; it uses the same timezone as the daily digest.

A week with nothing to report sends nothing. The daily digest already confirms Warden is alive; a weekly "no patterns this week" would just be another thing to learn to ignore.

The marker for "already sent this week" is stored in the database rather than in memory, so a restart neither re-sends nor skips.

## Configuration

| Setting | Default |
|---------|---------|
| Weekly summary enabled | false |
| Weekly summary day | Monday |
| Weekly summary time | 09:00 |

The detection window (14 days), the daily cadence and the detector thresholds are not configurable. They are chosen to be conservative: the cost of a false pattern is someone spending an afternoon chasing it.
