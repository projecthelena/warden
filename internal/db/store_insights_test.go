package db

import (
	"testing"
	"time"
)

func seedInsightMonitor(t *testing.T, s *Store, id, name string) {
	t.Helper()
	if err := s.CreateMonitor(Monitor{ID: id, GroupID: "g1", Name: name, URL: "https://" + id + ".example.com", Interval: 60}); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}
}

func TestReplaceMonitorInsights_ReplacesRatherThanAppends(t *testing.T) {
	RunTestWithBothDBs(t, "insights replace", func(t *testing.T, s *Store) {
		_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})
		seedInsightMonitor(t, s, "m1", "API")
		now := time.Now().UTC()

		first := []MonitorInsight{
			{Kind: "latency_sawtooth", Summary: "climbs and resets", Confidence: "high",
				Detail: map[string]any{"ramps": 9}},
			{Kind: "time_of_day", Summary: "evenings", Confidence: "medium"},
		}
		if err := s.ReplaceMonitorInsights("m1", first, now); err != nil {
			t.Fatalf("ReplaceMonitorInsights: %v", err)
		}

		got, err := s.GetMonitorInsights("m1")
		if err != nil {
			t.Fatalf("GetMonitorInsights: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(got))
		}

		// The detail survives the JSON round trip, since the UI shows its numbers.
		var sawtooth *MonitorInsight
		for i := range got {
			if got[i].Kind == "latency_sawtooth" {
				sawtooth = &got[i]
			}
		}
		if sawtooth == nil || sawtooth.Detail["ramps"] == nil {
			t.Fatalf("detail lost in the round trip: %+v", got)
		}
		if sawtooth.MonitorName != "API" {
			t.Errorf("monitor name not joined: %q", sawtooth.MonitorName)
		}

		// A later pass with one finding leaves one finding, not three.
		if err := s.ReplaceMonitorInsights("m1", first[:1], now.Add(24*time.Hour)); err != nil {
			t.Fatalf("second ReplaceMonitorInsights: %v", err)
		}
		got, _ = s.GetMonitorInsights("m1")
		if len(got) != 1 {
			t.Errorf("expected 1 finding after the replacement, got %d", len(got))
		}

		// And an empty pass clears them entirely, which is how a pattern that stopped
		// happening stops being reported.
		if err := s.ReplaceMonitorInsights("m1", nil, now.Add(48*time.Hour)); err != nil {
			t.Fatalf("clearing ReplaceMonitorInsights: %v", err)
		}
		got, _ = s.GetMonitorInsights("m1")
		if len(got) != 0 {
			t.Errorf("expected no findings, got %d", len(got))
		}
	})
}

func TestGetMonitorInsights_ScopesToOneMonitor(t *testing.T) {
	RunTestWithBothDBs(t, "insights scope", func(t *testing.T, s *Store) {
		_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})
		seedInsightMonitor(t, s, "m1", "Alpha")
		seedInsightMonitor(t, s, "m2", "Beta")
		now := time.Now().UTC()

		_ = s.ReplaceMonitorInsights("m1", []MonitorInsight{{Kind: "a", Summary: "one", Confidence: "high"}}, now)
		_ = s.ReplaceMonitorInsights("m2", []MonitorInsight{{Kind: "b", Summary: "two", Confidence: "high"}}, now)

		all, err := s.GetMonitorInsights("")
		if err != nil {
			t.Fatalf("GetMonitorInsights: %v", err)
		}
		if len(all) != 2 {
			t.Errorf("unfiltered returned %d, want 2", len(all))
		}

		mine, _ := s.GetMonitorInsights("m1")
		if len(mine) != 1 || mine[0].MonitorID != "m1" {
			t.Errorf("filtered returned %+v", mine)
		}
	})
}

// Hourly is the resolution the shapes live at: finer buries a four-hour ramp in per-check
// noise, coarser erases it entirely. And only successful checks count — a 10-second
// timeout would add 10,000ms to its hour, which is enough to manufacture a ramp and a
// reset out of an outage and hand the sawtooth detector a pattern that never happened.
func TestHourlyLatency(t *testing.T) {
	RunTestWithBothDBs(t, "hourly latency", func(t *testing.T, s *Store) {
		_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})
		seedInsightMonitor(t, s, "m1", "API")

		base := time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC)
		checks := []CheckResult{
			// Hour 10: two good checks and one long timeout. The average must ignore the
			// timeout entirely.
			{MonitorID: "m1", Status: "up", Latency: 100, Timestamp: base.Add(5 * time.Minute)},
			{MonitorID: "m1", Status: "up", Latency: 300, Timestamp: base.Add(25 * time.Minute)},
			{MonitorID: "m1", Status: "down", Latency: 10000, Timestamp: base.Add(45 * time.Minute)},
			// Hour 11: healthy.
			{MonitorID: "m1", Status: "up", Latency: 500, Timestamp: base.Add(time.Hour)},
			// Hour 12: nothing but failures.
			{MonitorID: "m1", Status: "down", Latency: 9000, Timestamp: base.Add(2 * time.Hour)},
		}
		if err := s.BatchInsertChecks(checks); err != nil {
			t.Fatalf("BatchInsertChecks: %v", err)
		}

		points, err := s.HourlyLatency("m1", base.Add(-time.Hour))
		if err != nil {
			t.Fatalf("HourlyLatency: %v", err)
		}
		if len(points) != 2 {
			t.Fatalf("expected 2 hourly points, got %d — an hour with no successful check should be omitted, not reported as zero", len(points))
		}
		if points[0].Latency != 200 {
			t.Errorf("first hour averaged %d, want 200 — the timeout leaked into the average", points[0].Latency)
		}
		if points[1].Latency != 500 {
			t.Errorf("second hour averaged %d, want 500", points[1].Latency)
		}

		// Anything before the window is excluded.
		points, _ = s.HourlyLatency("m1", base.Add(3*time.Hour))
		if len(points) != 0 {
			t.Errorf("expected nothing after the window, got %d points", len(points))
		}
	})
}

func TestEventTimesAndOutageWindows(t *testing.T) {
	RunTestWithBothDBs(t, "event times", func(t *testing.T, s *Store) {
		_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})
		seedInsightMonitor(t, s, "m1", "API")

		if err := s.CreateEvent("m1", "down", "Monitor is down"); err != nil {
			t.Fatalf("CreateEvent: %v", err)
		}
		if err := s.CreateEvent("m1", "recovered", "Monitor recovered"); err != nil {
			t.Fatalf("CreateEvent: %v", err)
		}

		since := time.Now().UTC().Add(-time.Hour)
		times, err := s.EventTimes("m1", []string{"down", "degraded"}, since)
		if err != nil {
			t.Fatalf("EventTimes: %v", err)
		}
		if len(times) != 1 {
			t.Errorf("expected only the down event, got %d", len(times))
		}
		if got, err := s.EventTimes("m1", nil, since); err != nil || len(got) != 0 {
			t.Errorf("an empty type list should return nothing: %v %v", got, err)
		}

		if err := s.CreateOutage("m1", "down", "Monitor is down"); err != nil {
			t.Fatalf("CreateOutage: %v", err)
		}
		windows, err := s.OutageWindowsSince(since)
		if err != nil {
			t.Fatalf("OutageWindowsSince: %v", err)
		}
		if len(windows) != 1 || windows[0].MonitorID != "m1" {
			t.Fatalf("OutageWindowsSince returned %+v", windows)
		}
		if windows[0].End != nil {
			t.Error("an open outage should have no end time")
		}

		if err := s.CloseOutage("m1"); err != nil {
			t.Fatalf("CloseOutage: %v", err)
		}
		windows, _ = s.OutageWindowsSince(since)
		if windows[0].End == nil {
			t.Error("a closed outage should carry its end time")
		}
	})
}
