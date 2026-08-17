package uptime

import (
	"fmt"
	"log"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/insights"
)

const (
	// Patterns are shapes over days, so once a day is plenty and keeps the scan off the
	// path of anything time-sensitive.
	insightsInterval = 24 * time.Hour
	// Two weeks is enough to see a weekly rhythm without letting a service that was fixed
	// a fortnight ago keep haunting the report.
	insightsWindowDays = 14
	// A pattern needs enough hours to be a pattern rather than a coincidence.
	insightsMinHours = 72
)

func (m *Manager) insightsWorker() {
	m.wg.Add(1)
	defer m.wg.Done()

	// The first pass runs a few minutes in, so a restart does not spend its first seconds
	// scanning history while monitors are still being scheduled.
	warmup := time.NewTimer(5 * time.Minute)
	defer warmup.Stop()

	select {
	case <-m.stopCh:
		return
	case <-warmup.C:
		m.refreshInsights(time.Now())
	}

	ticker := time.NewTicker(insightsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.refreshInsights(time.Now())
		}
	}
}

// refreshInsights runs every detector over every monitor and replaces the stored findings.
func (m *Manager) refreshInsights(now time.Time) {
	since := now.Add(-insightsWindowDays * 24 * time.Hour)

	windows, err := m.store.OutageWindowsSince(since)
	if err != nil {
		log.Printf("Insights: failed to load outage windows: %v", err)
		windows = nil
	}
	intervals := groupIntervals(windows)

	m.mu.RLock()
	loc := m.notificationTimezone
	m.mu.RUnlock()

	for id, mon := range m.GetAll() {
		findings := m.detectForMonitor(id, mon.GetName(), since, now, intervals, loc)
		if err := m.store.ReplaceMonitorInsights(id, findings, now); err != nil {
			log.Printf("Insights: failed to store findings for %s: %v", id, err)
		}
	}
}

// detectForMonitor is every detector applied to one monitor. Kept separate from the worker
// so a test can drive it directly with a seeded store.
func (m *Manager) detectForMonitor(id, name string, since, now time.Time, intervals map[string][]insights.Interval, loc *time.Location) []db.MonitorInsight {
	var out []db.MonitorInsight

	points, err := m.store.HourlyLatency(id, since)
	if err != nil {
		log.Printf("Insights: failed to load latency for %s: %v", id, err)
		return nil
	}

	samples := make([]insights.Sample, 0, len(points))
	for _, p := range points {
		samples = append(samples, insights.Sample{Hour: p.Timestamp, LatencyMs: p.Latency, HadFailure: p.Failed})
	}

	if len(samples) >= insightsMinHours {
		// Ramp and reset, plus whether the resets keep a schedule. Those are different
		// conclusions: a cadence points at a cron or a restart policy, an irregular one at
		// traffic, and only one of them is worth going to look for.
		if ramps, baseline, found := insights.DetectSawtooth(samples, insights.DefaultSawtoothConfig()); found {
			f := insights.SawtoothFinding(name, ramps, baseline, insightsWindowDays)
			out = append(out, toStored(id, f))

			peaks := make([]time.Time, 0, len(ramps))
			for _, r := range ramps {
				peaks = append(peaks, r.Peak)
			}
			if period, regular := insights.DetectPeriodicity(peaks, 3, 0.25); regular {
				out = append(out, toStored(id, insights.Finding{
					Kind: insights.KindPeriodicReset,
					Summary: fmt.Sprintf(
						"%s resets on a schedule, roughly every %s. A regular cadence like this usually means a timer rather than traffic — a cron, a restart policy, or a lease expiring.",
						name, formatAlertDuration(period)),
					Detail:     map[string]any{"periodHours": int(period.Hours())},
					Confidence: "medium",
				}))
			}
		}

		// Week over week: the slow slide that never trips a threshold because every day
		// looks like the one before it.
		half := len(samples) / 2
		if pct, recent, prev, found := insights.DetectDrift(samples[half:], samples[:half], 25, 20); found {
			direction := "slower"
			if pct < 0 {
				direction = "faster"
			}
			out = append(out, toStored(id, insights.Finding{
				Kind: insights.KindLatencyDrift,
				Summary: fmt.Sprintf(
					"%s is %.0f%% %s than it was a week ago: a typical response went from %dms to %dms. Nothing alerted, because no single check was slow enough to.",
					name, abs(pct), direction, prev, recent),
				Detail: map[string]any{
					"changePercent":    int(pct),
					"recentMedianMs":   recent,
					"previousMedianMs": prev,
				},
				Confidence: "high",
			}))
		}
	}

	// When trouble happens, rather than how bad it is.
	times, err := m.store.EventTimes(id, []string{"down", "degraded"}, since)
	if err != nil {
		log.Printf("Insights: failed to load event times for %s: %v", id, err)
	} else if start, width, share, found := insights.DetectTimeOfDay(times, 8, 8, 0.6); found {
		out = append(out, toStored(id, insights.TimeOfDayFinding(name, start, width, share, len(times), loc)))
	}

	// Monitors that fail together share a cause and should be investigated together.
	if mine := intervals[id]; len(mine) >= 3 {
		for otherID, theirs := range intervals {
			if otherID == id || len(theirs) < 3 {
				continue
			}
			overlap := insights.Overlap(mine, theirs, now)
			if overlap < 0.7 {
				continue
			}
			otherName := otherID
			if other := m.GetMonitor(otherID); other != nil {
				otherName = other.GetName()
			}
			out = append(out, toStored(id, insights.Finding{
				Kind: insights.KindCoFailure,
				Summary: fmt.Sprintf(
					"%s and %s go down together: %.0f%% of %s's downtime in the last %d days overlapped with %s. They almost certainly share a cause, so treat them as one thing.",
					name, otherName, overlap*100, name, insightsWindowDays, otherName),
				Detail:     map[string]any{"withMonitorId": otherID, "overlap": int(overlap * 100)},
				Confidence: "medium",
			}))
		}
	}

	return out
}

func toStored(monitorID string, f insights.Finding) db.MonitorInsight {
	return db.MonitorInsight{
		MonitorID:  monitorID,
		Kind:       string(f.Kind),
		Summary:    f.Summary,
		Detail:     f.Detail,
		Confidence: f.Confidence,
	}
}

func groupIntervals(windows []db.OutageWindow) map[string][]insights.Interval {
	out := make(map[string][]insights.Interval)
	for _, w := range windows {
		iv := insights.Interval{Start: w.Start}
		if w.End != nil {
			iv.End = *w.End
		}
		out[w.MonitorID] = append(out[w.MonitorID], iv)
	}
	return out
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
