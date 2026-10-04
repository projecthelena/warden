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

	running := m.GetAll()
	for id, mon := range running {
		findings := m.detectForMonitor(id, mon.GetName(), since, now, intervals, loc)
		if err := m.store.ReplaceMonitorInsights(id, findings, now); err != nil {
			log.Printf("Insights: failed to store findings for %s: %v", id, err)
		}
	}

	// A paused monitor is not in the running set, so its findings would otherwise sit
	// there forever — still on its page, still counted in the weekly summary, describing
	// a fortnight that keeps receding. Nothing is being measured, so there is nothing to
	// report.
	monitors, err := m.store.GetMonitors()
	if err != nil {
		log.Printf("Insights: failed to load monitors: %v", err)
		return
	}
	for _, dbM := range monitors {
		if _, isRunning := running[dbM.ID]; isRunning {
			continue
		}
		if err := m.store.ReplaceMonitorInsights(dbM.ID, nil, now); err != nil {
			log.Printf("Insights: failed to clear findings for %s: %v", dbM.ID, err)
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
		samples = append(samples, insights.Sample{Hour: p.Timestamp, LatencyMs: p.Latency})
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
						"%s resets on a schedule, roughly every %s. Compare this cadence with scheduled jobs and restart history; the cause is not confirmed.",
						name, formatAlertDuration(period)),
					Detail:     map[string]any{"periodHours": int(period.Hours())},
					Confidence: "medium",
				}))
			}
		}

		// Week over week: the slow slide that never trips a threshold because every day
		// looks like the one before it.
		var recentSamples, previousSamples []insights.Sample
		boundary := now.Add(-7 * 24 * time.Hour)
		for _, sample := range samples {
			if sample.Hour.Before(boundary) {
				previousSamples = append(previousSamples, sample)
			} else {
				recentSamples = append(recentSamples, sample)
			}
		}
		if pct, recent, prev, found := insights.DetectDrift(recentSamples, previousSamples, 25, 20); found && len(recentSamples) >= insightsMinHours && len(previousSamples) >= insightsMinHours {
			kind := insights.KindLatencyDrift
			summary := fmt.Sprintf(
				"%s is %.0f%% slower than it was a week ago: a typical response went from %dms to %dms. Compare HTTP timings between these periods to locate the slowdown.",
				name, pct, prev, recent)
			if pct < 0 {
				// Worth saying out loud: it is how you find out a fix landed.
				kind = insights.KindLatencyImproved
				summary = fmt.Sprintf(
					"%s is %.0f%% faster than it was a week ago: a typical response went from %dms to %dms.",
					name, abs(pct), prev, recent)
			}
			out = append(out, toStored(id, insights.Finding{
				Kind:    kind,
				Summary: summary,
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
		out = append(out, toStored(id, insights.TimeOfDayFinding(name, start, width, share, len(times), loc, now)))
	}

	// Repeated simultaneous starts and mutual overlap are evidence of co-failure, not causation.
	if mine := intervals[id]; len(mine) >= 3 {
		for otherID, theirs := range intervals {
			if otherID == id || len(theirs) < 3 {
				continue
			}
			overlap := insights.Overlap(mine, theirs, now)
			reverseOverlap := insights.Overlap(theirs, mine, now)
			matched := insights.CoincidentStarts(mine, theirs, now, 5*time.Minute)
			if overlap < 0.7 || reverseOverlap < 0.7 || matched < 3 {
				continue
			}
			otherName := otherID
			if other := m.GetMonitor(otherID); other != nil {
				otherName = other.GetName()
			}
			out = append(out, toStored(id, insights.Finding{
				Kind: insights.KindCoFailure,
				Summary: fmt.Sprintf(
					"%s and %s go down together: %d distinct outages started within 5 minutes of each other. %.0f%% of %s's downtime in the last %d days overlapped with %s (%.0f%% in the other direction). A shared cause is not confirmed; compare their failed checks.",
					name, otherName, matched, overlap*100, name, insightsWindowDays, otherName, reverseOverlap*100),
				Detail:     map[string]any{"withMonitorId": otherID, "withMonitorName": otherName, "overlap": int(overlap * 100), "reverseOverlap": int(reverseOverlap * 100), "matchedOutages": matched},
				Confidence: "medium",
			}))
		}
	}

	if len(out) > 0 {
		summary, err := m.store.GetMonitorCheckSummary(id, since, now)
		if err != nil {
			log.Printf("Insights: failed to load trace evidence for %s: %v", id, err)
		} else {
			for i := range out {
				out[i].Detail["checkEvidence"] = summary
			}
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
