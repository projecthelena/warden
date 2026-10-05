package uptime

import (
	"strings"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/insights"
)

// prod3Hourly is the real hourly latency of homedepot-nucleus-prod-3 for a week in August
// 2026, taken from the live instance. Seeding it end to end proves the detectors survive
// the trip through the database, not just the unit tests in internal/insights.
var prod3Hourly = []int64{
	275, 255, 254, 288, 302, 417, 562, 399, 450, 438, 501, 410, 758, 255, 255, 258, 250, 251,
	257, 248, 252, 267, 271, 365, 257, 262, 256, 253, 250, 261, 255, 255, 262, 264, 305, 339,
	315, 255, 279, 385, 463, 269, 734, 254, 251, 249, 256, 253, 255, 254, 265, 254, 254, 246,
	255, 250, 252, 253, 257, 259, 252, 249, 252, 254, 324, 252, 622, 265, 325, 254, 254, 258,
	256, 253, 251, 251, 262, 255, 250, 251, 257, 257, 277, 267, 280, 327, 270, 259, 257, 266,
	266, 652, 256, 270, 257, 275, 305, 322, 253, 256, 252, 255, 257, 258, 252, 259, 257, 256,
	260, 260, 665, 263, 267, 265, 310, 456, 418, 280, 320, 253, 254, 256, 252, 252, 256, 254,
	261, 257, 256, 253, 278, 276, 285, 323, 270, 252, 254, 277, 263, 285, 294, 324, 426, 326,
	646, 251, 249, 257, 258, 256, 259, 259, 261, 273, 306, 260, 255, 255, 258, 256, 252, 253,
	253, 337, 278, 256, 252, 263, 275,
}

// seedHourly writes six checks per hour at the given latency, which is what the hourly
// average is computed from.
func seedHourly(t *testing.T, store *db.Store, monitorID string, values []int64, endingAt time.Time) {
	t.Helper()
	start := endingAt.Add(-time.Duration(len(values)) * time.Hour)

	var checks []db.CheckResult
	for i, v := range values {
		hour := start.Add(time.Duration(i) * time.Hour)
		for k := 0; k < 6; k++ {
			checks = append(checks, db.CheckResult{
				MonitorID: monitorID, Status: "up", Latency: v,
				Timestamp: hour.Add(time.Duration(k*10) * time.Minute),
			})
		}
	}
	if err := store.BatchInsertChecks(checks); err != nil {
		t.Fatalf("BatchInsertChecks: %v", err)
	}
}

func findingsByKind(findings []db.MonitorInsight, kind insights.Kind) []db.MonitorInsight {
	var out []db.MonitorInsight
	for _, f := range findings {
		if f.Kind == string(kind) {
			out = append(out, f)
		}
	}
	return out
}

// The whole point of the phase, end to end: the shape Jesus suspected from squinting at
// charts comes back out of the database as a sentence.
func TestDetectForMonitor_FindsTheSawtoothInRealData(t *testing.T) {
	m, store := newAlertTestManager(t)
	now := time.Now().UTC().Truncate(time.Hour)
	seedHourly(t, store, "m1", prod3Hourly, now)

	findings := m.detectForMonitor("m1", "prod-3", now.Add(-14*24*time.Hour), now, nil, time.UTC)

	saw := findingsByKind(findings, insights.KindSawtooth)
	if len(saw) != 1 {
		t.Fatalf("expected 1 sawtooth finding, got %d (%d findings total)", len(saw), len(findings))
	}
	if !strings.Contains(saw[0].Summary, "climbs and resets") {
		t.Errorf("summary does not describe the shape: %q", saw[0].Summary)
	}
	if ramps, ok := saw[0].Detail["ramps"].(int); !ok || ramps < 5 {
		t.Errorf("detail lost the ramp count: %+v", saw[0].Detail)
	}

	// The peaks are 6h, 11h, 12h, 5h, 19h, 21h, 27h apart, so this is load-driven rather
	// than scheduled. Claiming a cadence here would send someone hunting a cron that does
	// not exist.
	if got := findingsByKind(findings, insights.KindPeriodicReset); len(got) != 0 {
		t.Errorf("irregular peaks were reported as a schedule: %q", got[0].Summary)
	}
}

// A service that is flat and healthy must produce nothing at all. A detector that always
// finds something is a detector nobody reads.
func TestDetectForMonitor_QuietMonitorProducesNothing(t *testing.T) {
	m, store := newAlertTestManager(t)
	now := time.Now().UTC().Truncate(time.Hour)

	flat := make([]int64, 168)
	for i := range flat {
		flat[i] = 250
	}
	seedHourly(t, store, "m1", flat, now)

	findings := m.detectForMonitor("m1", "steady", now.Add(-14*24*time.Hour), now, nil, time.UTC)
	if len(findings) != 0 {
		t.Errorf("a healthy monitor produced %d findings: %+v", len(findings), findings)
	}
}

// A regular cadence is a different conclusion from an irregular one: it points at a timer
// rather than at traffic.
func TestDetectForMonitor_ReportsAScheduleWhenThereIsOne(t *testing.T) {
	m, store := newAlertTestManager(t)
	now := time.Now().UTC().Truncate(time.Hour)

	// Climb for five hours, reset, idle for one: a clean six-hour cycle.
	var series []int64
	for cycle := 0; cycle < 28; cycle++ {
		series = append(series, 200, 260, 330, 400, 470, 200)
	}
	seedHourly(t, store, "m1", series, now)

	findings := m.detectForMonitor("m1", "leaky", now.Add(-14*24*time.Hour), now, nil, time.UTC)

	if got := findingsByKind(findings, insights.KindSawtooth); len(got) != 1 {
		t.Fatalf("expected a sawtooth finding, got %d", len(got))
	}
	periodic := findingsByKind(findings, insights.KindPeriodicReset)
	if len(periodic) != 1 {
		t.Fatalf("a clean 6-hour cycle was not reported as a schedule (%d findings)", len(periodic))
	}
	if !strings.Contains(periodic[0].Summary, "6h") {
		t.Errorf("summary does not name the period: %q", periodic[0].Summary)
	}
	for _, kind := range []insights.Kind{insights.KindSawtooth, insights.KindPeriodicReset} {
		finding := findingsByKind(findings, kind)[0]
		ref, ok := finding.Detail["evidenceCheck"].(*db.InsightCheck)
		if !ok || ref.HasTrace || !ref.Timestamp.Equal(now.Add(-70*time.Minute)) {
			t.Fatalf("%s did not reference the last peak hour: %+v", kind, finding.Detail)
		}
		checks, err := store.GetMonitorCheck("m1", ref.ID)
		if err != nil || len(checks) != 1 || checks[0].Latency != 470 || checks[0].Status != "up" {
			t.Fatalf("%s linked the reset instead of the peak: %+v %v", kind, checks, err)
		}
	}
}

// The slow slide that never trips a threshold, because every day looks like the one before.
func TestDetectForMonitor_ReportsDrift(t *testing.T) {
	m, store := newAlertTestManager(t)
	now := time.Now().UTC().Truncate(time.Hour)

	series := make([]int64, 0, 336)
	for i := 0; i < 168; i++ {
		series = append(series, 250)
	}
	for i := 0; i < 168; i++ {
		series = append(series, 400)
	}
	seedHourly(t, store, "m1", series, now)

	findings := m.detectForMonitor("m1", "creeping", now.Add(-14*24*time.Hour), now, nil, time.UTC)

	drift := findingsByKind(findings, insights.KindLatencyDrift)
	if len(drift) != 1 {
		t.Fatalf("a 60%% week-over-week slowdown was missed (%d findings)", len(findings))
	}
	for key, recent := range map[string]bool{"evidenceCheck": true, "comparisonCheck": false} {
		ref, ok := drift[0].Detail[key].(*db.InsightCheck)
		if !ok || ref == nil || ref.HasTrace {
			t.Fatalf("missing untraced reference %s: %+v", key, drift[0].Detail)
		}
		if ref.Timestamp.Before(now.Add(-7*24*time.Hour)) == recent {
			t.Fatalf("%s belongs to wrong week: %+v", key, ref)
		}
	}
	if !strings.Contains(drift[0].Summary, "slower") {
		t.Errorf("summary does not name the direction: %q", drift[0].Summary)
	}
	if strings.Contains(drift[0].Summary, "Nothing alerted") {
		t.Errorf("summary must not invent alert history: %q", drift[0].Summary)
	}
}

func TestDetectForMonitor_ReportsCoFailure(t *testing.T) {
	m, _ := newAlertTestManager(t)
	now := time.Now().UTC().Truncate(time.Hour)

	base := now.Add(-48 * time.Hour)
	end := func(t time.Time) *time.Time { return &t }

	intervals := groupIntervals([]db.OutageWindow{
		{MonitorID: "m1", Start: base, End: end(base.Add(10 * time.Minute))},
		{MonitorID: "m1", Start: base.Add(time.Hour), End: end(base.Add(70 * time.Minute))},
		{MonitorID: "m1", Start: base.Add(2 * time.Hour), End: end(base.Add(130 * time.Minute))},
		{MonitorID: "m2", Start: base, End: end(base.Add(10 * time.Minute))},
		{MonitorID: "m2", Start: base.Add(time.Hour), End: end(base.Add(70 * time.Minute))},
		{MonitorID: "m2", Start: base.Add(2 * time.Hour), End: end(base.Add(130 * time.Minute))},
	})

	findings := m.detectForMonitor("m1", "api", now.Add(-14*24*time.Hour), now, intervals, time.UTC)

	co := findingsByKind(findings, insights.KindCoFailure)
	if len(co) != 1 {
		t.Fatalf("two monitors failing in lockstep were not linked (%d findings)", len(findings))
	}
	if !strings.Contains(co[0].Summary, "not confirmed") {
		t.Errorf("summary must distinguish overlap from causation: %q", co[0].Summary)
	}
	if co[0].Detail["withMonitorId"] != "m2" {
		t.Errorf("detail does not name the other monitor: %+v", co[0].Detail)
	}
}

// Findings are replaced wholesale rather than appended, so a pattern that stops happening
// stops being reported and a daily re-run does not pile up duplicates. A stale finding is
// worse than none: it sends someone looking for something that is no longer there.
func TestRefreshInsights_ReplacesRatherThanAppends(t *testing.T) {
	m, store := newAlertTestManager(t)
	now := time.Now().UTC().Truncate(time.Hour)
	seedHourly(t, store, "m1", prod3Hourly, now)

	m.refreshInsights(now)
	first, err := store.GetMonitorInsights("m1")
	if err != nil {
		t.Fatalf("GetMonitorInsights: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("no findings stored on the first pass")
	}

	// A day later, same history, same conclusions — not twice as many rows.
	m.refreshInsights(now.Add(24 * time.Hour))
	second, err := store.GetMonitorInsights("m1")
	if err != nil {
		t.Fatalf("GetMonitorInsights: %v", err)
	}
	if len(second) != len(first) {
		t.Errorf("findings went from %d to %d across two passes — they are accumulating", len(first), len(second))
	}
}

// A monitor with nothing to say stores nothing, even while a noisy neighbour stores plenty.
func TestRefreshInsights_IsPerMonitor(t *testing.T) {
	m, store := newAlertTestManager(t)
	now := time.Now().UTC().Truncate(time.Hour)

	if err := store.CreateMonitor(db.Monitor{
		ID: "m2", GroupID: "g1", Name: "Steady", URL: "https://steady.example.com",
		Active: true, Interval: 60,
	}); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}
	m.monitors["m2"] = NewMonitor("m2", db.MonitorTypeHTTP, "g1", "Steady",
		"https://steady.example.com", time.Minute, m.jobQueue, time.Now(), nil)

	seedHourly(t, store, "m1", prod3Hourly, now)
	flat := make([]int64, 200)
	for i := range flat {
		flat[i] = 250
	}
	seedHourly(t, store, "m2", flat, now)

	m.refreshInsights(now)

	noisy, _ := store.GetMonitorInsights("m1")
	if len(noisy) == 0 {
		t.Error("the monitor with a real pattern produced no findings")
	}
	steady, _ := store.GetMonitorInsights("m2")
	if len(steady) != 0 {
		t.Errorf("a healthy monitor produced findings: %+v", steady)
	}

	all, _ := store.GetMonitorInsights("")
	if len(all) != len(noisy) {
		t.Errorf("the unfiltered listing returned %d, want %d", len(all), len(noisy))
	}
}

// A paused monitor is not being measured, so it has nothing to report. Leaving its old
// findings in place would keep them on its page and in the weekly summary, describing a
// fortnight that keeps receding further into the past.
func TestRefreshInsights_ClearsFindingsForPausedMonitors(t *testing.T) {
	m, store := newAlertTestManager(t)
	now := time.Now().UTC().Truncate(time.Hour)
	seedHourly(t, store, "m1", prod3Hourly, now)

	m.refreshInsights(now)
	if got, _ := store.GetMonitorInsights("m1"); len(got) == 0 {
		t.Fatal("no findings stored while the monitor was running")
	}

	// Pausing takes it out of the running set, exactly as Sync does.
	delete(m.monitors, "m1")
	m.refreshInsights(now)

	if got, _ := store.GetMonitorInsights("m1"); len(got) != 0 {
		t.Errorf("a paused monitor kept %d stale findings: %+v", len(got), got)
	}
}

func TestDetectForMonitor_RejectsChronicNeighbor(t *testing.T) {
	m, _ := newAlertTestManager(t)
	now := time.Now().UTC().Truncate(time.Hour)
	base := now.Add(-48 * time.Hour)
	mine := []insights.Interval{}
	for i := 0; i < 3; i++ {
		start := base.Add(time.Duration(i) * time.Hour)
		mine = append(mine, insights.Interval{Start: start, End: start.Add(10 * time.Minute)})
	}
	theirs := []insights.Interval{{Start: base.Add(-time.Hour), End: base.Add(4 * time.Hour)}, {Start: base.Add(-24 * time.Hour), End: base.Add(-23 * time.Hour)}, {Start: base.Add(-20 * time.Hour), End: base.Add(-19 * time.Hour)}}
	findings := m.detectForMonitor("m1", "API", now.Add(-14*24*time.Hour), now, map[string][]insights.Interval{"m1": mine, "m2": theirs}, time.UTC)
	if got := findingsByKind(findings, insights.KindCoFailure); len(got) != 0 {
		t.Fatalf("chronic neighbor reported: %+v", got)
	}
}

func TestDetectForMonitor_DoesNotCallThreeDaysAWeek(t *testing.T) {
	m, store := newAlertTestManager(t)
	now := time.Now().UTC().Truncate(time.Hour)
	series := make([]int64, 100)
	for i := range series {
		if i < 50 {
			series[i] = 250
		} else {
			series[i] = 400
		}
	}
	seedHourly(t, store, "m1", series, now)
	findings := m.detectForMonitor("m1", "API", now.Add(-14*24*time.Hour), now, nil, time.UTC)
	if got := findingsByKind(findings, insights.KindLatencyDrift); len(got) != 0 {
		t.Fatalf("partial week reported as week-over-week: %+v", got)
	}
}

func TestCoFailureLinksChecksFromTheSameMatchedOutage(t *testing.T) {
	m, store := newAlertTestManager(t)
	if err := store.CreateMonitor(db.Monitor{ID: "m2", GroupID: "g1", Name: "Peer", URL: "https://example.test", Interval: 60}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Hour)
	intervals := map[string][]insights.Interval{}
	for _, id := range []string{"m1", "m2"} {
		for _, age := range []int{72, 48, 24} {
			start := now.Add(-time.Duration(age) * time.Hour)
			intervals[id] = append(intervals[id], insights.Interval{Start: start, End: start.Add(10 * time.Minute)})
			if err := store.BatchInsertChecks([]db.CheckResult{{MonitorID: id, Timestamp: start.Add(time.Minute), Status: "down", Latency: 500}}); err != nil {
				t.Fatal(err)
			}
		}
	}
	// More recent failures outside the paired episodes must not replace evidence.
	if err := store.BatchInsertChecks([]db.CheckResult{{MonitorID: "m1", Timestamp: now.Add(-time.Hour), Status: "down"}}); err != nil {
		t.Fatal(err)
	}
	findings := m.detectForMonitor("m1", "API", now.Add(-14*24*time.Hour), now, intervals, time.UTC)
	co := findingsByKind(findings, insights.KindCoFailure)
	if len(co) != 1 {
		t.Fatalf("missing co-failure: %+v", findings)
	}
	for key, id := range map[string]string{"evidenceCheck": "m1", "withCheck": "m2"} {
		ref, ok := co[0].Detail[key].(*db.InsightCheck)
		if !ok || ref == nil || ref.HasTrace || !ref.Timestamp.Equal(now.Add(-24*time.Hour+time.Minute)) {
			t.Fatalf("wrong %s reference: %+v", key, co[0].Detail)
		}
		rows, err := store.GetMonitorCheck(id, ref.ID)
		if err != nil || len(rows) != 1 || rows[0].Status != "down" {
			t.Fatalf("reference does not resolve to %s: %+v %v", id, rows, err)
		}
	}
	// Retention can remove raw checks without deleting outage history.
	if err := store.PruneMonitorChecks(1); err != nil {
		t.Fatal(err)
	}
	findings = m.detectForMonitor("m1", "API", now.Add(-14*24*time.Hour), now, intervals, time.UTC)
	co = findingsByKind(findings, insights.KindCoFailure)
	if len(co) != 1 || co[0].Detail["evidenceCheck"] != nil || co[0].Detail["withCheck"] != nil {
		t.Fatalf("expired evidence was replaced: %+v", co)
	}
}

func TestTimeOfDayLinksAFailedCheckWithoutRequiringTraces(t *testing.T) {
	for _, status := range []string{"down", "degraded"} {
		t.Run(status, func(t *testing.T) {
			m, store := newAlertTestManager(t)
			for i := 0; i < 8; i++ {
				if err := store.CreateEvent("m1", status, "Example event"); err != nil {
					t.Fatal(err)
				}
			}
			now := time.Now().UTC()
			if err := store.BatchInsertChecks([]db.CheckResult{
				{MonitorID: "m1", Timestamp: now, Status: status, Latency: 300},
				{MonitorID: "m1", Timestamp: now.Add(time.Minute), Status: "up", Latency: 200},
			}); err != nil {
				t.Fatal(err)
			}
			findings := findingsByKind(m.detectForMonitor("m1", "Example", now.Add(-14*24*time.Hour), now, nil, time.UTC), insights.KindTimeOfDay)
			if len(findings) != 1 {
				t.Fatalf("expected time-of-day finding: %+v", findings)
			}
			ref, ok := findings[0].Detail["evidenceCheck"].(*db.InsightCheck)
			if !ok || ref.HasTrace {
				t.Fatalf("untraced failure unavailable: %+v", findings[0].Detail)
			}
			checks, err := store.GetMonitorCheck("m1", ref.ID)
			if err != nil || len(checks) != 1 || checks[0].Status != status {
				t.Fatalf("linked a successful check: %+v %v", checks, err)
			}
		})
	}
}
