package db

import (
	"testing"
	"time"
)

func TestGetLatencyChart_FillsGapsAndSeparatesFailures(t *testing.T) {
	RunTestWithBothDBs(t, "latency chart", func(t *testing.T, s *Store) {
		now := time.Date(2026, 9, 17, 10, 30, 0, 0, time.UTC)
		if err := s.CreateGroup(Group{ID: "g-chart", Name: "Chart"}); err != nil {
			t.Fatal(err)
		}
		if err := s.CreateMonitor(Monitor{ID: "m-chart", GroupID: "g-chart", Name: "Chart monitor", URL: "https://example.test", Active: true, Interval: 60, CreatedAt: now.Add(-48 * time.Hour)}); err != nil {
			t.Fatal(err)
		}

		checks := []CheckResult{
			{MonitorID: "m-chart", Status: "up", Latency: 100, Timestamp: now.Add(-3*time.Hour + 5*time.Minute), StatusCode: 200},
			{MonitorID: "m-chart", Status: "up", Latency: 300, Timestamp: now.Add(-3*time.Hour + 20*time.Minute), StatusCode: 200},
			{MonitorID: "m-chart", Status: "down", Latency: 8, Timestamp: now.Add(-2*time.Hour + 10*time.Minute), StatusCode: 503},
			{MonitorID: "m-chart", Status: "up", Latency: 400, Timestamp: now.Add(-time.Hour + 5*time.Minute), StatusCode: 200},
			{MonitorID: "m-chart", Status: "down", Latency: 10_000, Timestamp: now.Add(-time.Hour + 15*time.Minute), StatusCode: 0},
		}
		if err := s.BatchInsertChecks(checks); err != nil {
			t.Fatal(err)
		}

		points, err := s.GetLatencyChart("m-chart", 24, now)
		if err != nil {
			t.Fatal(err)
		}
		if len(points) != 23 {
			t.Fatalf("got %d buckets, want 23 complete buckets", len(points))
		}
		if points[0].Timestamp != now.Add(-23*time.Hour).Truncate(time.Hour) {
			t.Fatalf("first bucket is partial: %s", points[0].Timestamp)
		}
		if points[len(points)-1].Timestamp != now.Add(-time.Hour).Truncate(time.Hour) {
			t.Fatalf("last bucket is partial: %s", points[len(points)-1].Timestamp)
		}

		byTime := make(map[time.Time]LatencyChartPoint, len(points))
		for _, point := range points {
			byTime[point.Timestamp] = point
		}

		up := byTime[now.Add(-3*time.Hour).Truncate(time.Hour)]
		if up.State != "up" || up.Latency == nil || *up.Latency != 200 || up.SuccessfulChecks != 2 || up.FailedChecks != 0 {
			t.Fatalf("unexpected up bucket: %+v", up)
		}

		down := byTime[now.Add(-2*time.Hour).Truncate(time.Hour)]
		if down.State != "down" || down.Latency != nil || down.SuccessfulChecks != 0 || down.FailedChecks != 1 {
			t.Fatalf("failed latency must not become response latency: %+v", down)
		}

		mixed := byTime[now.Add(-time.Hour).Truncate(time.Hour)]
		if mixed.State != "mixed" || mixed.Latency == nil || *mixed.Latency != 400 || mixed.SuccessfulChecks != 1 || mixed.FailedChecks != 1 {
			t.Fatalf("unexpected mixed bucket: %+v", mixed)
		}

		gap := byTime[now.Add(-4*time.Hour).Truncate(time.Hour)]
		if gap.State != "no_data" || gap.Latency != nil || gap.TotalChecks != 0 {
			t.Fatalf("missing bucket was not preserved as no data: %+v", gap)
		}
	})
}

func TestGetLatencyChart_IncludesObservedCurrentBucket(t *testing.T) {
	RunTestWithBothDBs(t, "latency chart current bucket", func(t *testing.T, s *Store) {
		now := time.Date(2026, 9, 17, 10, 30, 30, 0, time.UTC)
		if err := s.CreateGroup(Group{ID: "g-chart-current", Name: "Chart"}); err != nil {
			t.Fatal(err)
		}
		if err := s.CreateMonitor(Monitor{ID: "m-chart-current", GroupID: "g-chart-current", Name: "Chart monitor", URL: "https://example.test", Active: true, Interval: 60}); err != nil {
			t.Fatal(err)
		}
		if err := s.BatchInsertChecks([]CheckResult{{MonitorID: "m-chart-current", Status: "up", Latency: 125, Timestamp: now.Add(-5 * time.Second), StatusCode: 200}}); err != nil {
			t.Fatal(err)
		}

		points, err := s.GetLatencyChart("m-chart-current", 1, now)
		if err != nil {
			t.Fatal(err)
		}
		last := points[len(points)-1]
		if last.Timestamp != now.Truncate(time.Minute) || last.State != "up" {
			t.Fatalf("freshest observed bucket missing: %+v", last)
		}
	})
}

func TestGetLatencyChart_RejectsInvalidRange(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.GetLatencyChart("m1", 0, time.Now()); err == nil {
		t.Fatal("expected invalid range error")
	}
}
