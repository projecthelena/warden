package db

import (
	"testing"
	"time"
)

func seedBaselineMonitor(t *testing.T, s *Store) {
	t.Helper()
	if err := s.CreateGroup(Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if err := s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "API", URL: "https://api.example.com", Interval: 60}); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}
}

// The percentiles are computed with ORDER BY + OFFSET rather than a percentile function,
// because SQLite has none. This is the test that says the same query means the same thing
// on both databases.
func TestComputeLatencyBaseline_Percentiles(t *testing.T) {
	RunTestWithBothDBs(t, "latency baseline", func(t *testing.T, s *Store) {
		seedBaselineMonitor(t, s)
		now := time.Now().UTC()

		// 1..100 ms, one per minute. p50 lands on 51, p95 on 96.
		var checks []CheckResult
		for i := 1; i <= 100; i++ {
			checks = append(checks, CheckResult{
				MonitorID: "m1", Status: "up", Latency: int64(i),
				Timestamp: now.Add(-time.Duration(i) * time.Minute),
			})
		}
		if err := s.BatchInsertChecks(checks); err != nil {
			t.Fatalf("BatchInsertChecks: %v", err)
		}

		written, err := s.ComputeLatencyBaseline("m1", 7*24*time.Hour, 10, now)
		if err != nil || !written {
			t.Fatalf("ComputeLatencyBaseline: written=%v err=%v", written, err)
		}

		b, ok, err := s.GetLatencyBaseline("m1")
		if err != nil || !ok {
			t.Fatalf("GetLatencyBaseline: ok=%v err=%v", ok, err)
		}
		if b.Samples != 100 {
			t.Errorf("samples = %d, want 100", b.Samples)
		}
		if b.P50 != 51 {
			t.Errorf("p50 = %d, want 51", b.P50)
		}
		if b.P95 != 96 {
			t.Errorf("p95 = %d, want 96", b.P95)
		}
	})
}

func TestComputeLatencyBaseline_Upserts(t *testing.T) {
	RunTestWithBothDBs(t, "baseline upsert", func(t *testing.T, s *Store) {
		seedBaselineMonitor(t, s)
		now := time.Now().UTC()

		var checks []CheckResult
		for i := 0; i < 20; i++ {
			checks = append(checks, CheckResult{
				MonitorID: "m1", Status: "up", Latency: 100,
				Timestamp: now.Add(-time.Duration(i) * time.Minute),
			})
		}
		if err := s.BatchInsertChecks(checks); err != nil {
			t.Fatalf("BatchInsertChecks: %v", err)
		}
		if _, err := s.ComputeLatencyBaseline("m1", 7*24*time.Hour, 10, now); err != nil {
			t.Fatalf("first compute: %v", err)
		}

		// The service gets slower; recomputing must replace the row, not add a second one.
		var slower []CheckResult
		for i := 0; i < 40; i++ {
			slower = append(slower, CheckResult{
				MonitorID: "m1", Status: "up", Latency: 300,
				Timestamp: now.Add(-time.Duration(i) * time.Second),
			})
		}
		if err := s.BatchInsertChecks(slower); err != nil {
			t.Fatalf("BatchInsertChecks: %v", err)
		}
		if _, err := s.ComputeLatencyBaseline("m1", 7*24*time.Hour, 10, now); err != nil {
			t.Fatalf("second compute: %v", err)
		}

		all, err := s.GetLatencyBaselines()
		if err != nil {
			t.Fatalf("GetLatencyBaselines: %v", err)
		}
		if len(all) != 1 {
			t.Fatalf("expected 1 baseline row, got %d", len(all))
		}
		if all["m1"].Samples != 60 {
			t.Errorf("samples = %d, want 60", all["m1"].Samples)
		}
		if all["m1"].P95 != 300 {
			t.Errorf("p95 = %d, want 300 after the service slowed down", all["m1"].P95)
		}
	})
}

// Checks outside the window must not drag the baseline: that is the whole point of a
// *rolling* baseline.
func TestComputeLatencyBaseline_RespectsWindow(t *testing.T) {
	RunTestWithBothDBs(t, "baseline window", func(t *testing.T, s *Store) {
		seedBaselineMonitor(t, s)
		now := time.Now().UTC()

		var checks []CheckResult
		for i := 0; i < 30; i++ {
			// Recent and fast.
			checks = append(checks, CheckResult{
				MonitorID: "m1", Status: "up", Latency: 100,
				Timestamp: now.Add(-time.Duration(i) * time.Minute),
			})
			// Old and slow, well outside a one-day window.
			checks = append(checks, CheckResult{
				MonitorID: "m1", Status: "up", Latency: 9000,
				Timestamp: now.AddDate(0, 0, -30).Add(-time.Duration(i) * time.Minute),
			})
		}
		if err := s.BatchInsertChecks(checks); err != nil {
			t.Fatalf("BatchInsertChecks: %v", err)
		}

		if _, err := s.ComputeLatencyBaseline("m1", 24*time.Hour, 10, now); err != nil {
			t.Fatalf("ComputeLatencyBaseline: %v", err)
		}
		b, _, _ := s.GetLatencyBaseline("m1")
		if b.Samples != 30 || b.P95 != 100 {
			t.Errorf("old checks leaked into the window: samples=%d p95=%d", b.Samples, b.P95)
		}
	})
}

func TestRelearnLatencyBaselines_KeepsHistoryAndExcludesOldChecks(t *testing.T) {
	RunTestWithBothDBs(t, "relearn latency baseline", func(t *testing.T, s *Store) {
		seedBaselineMonitor(t, s)
		now := time.Now().UTC().Truncate(time.Second)
		old := make([]CheckResult, 20)
		for i := range old {
			old[i] = CheckResult{MonitorID: "m1", Status: "up", Latency: 100, Timestamp: now.Add(-time.Hour)}
		}
		if err := s.BatchInsertChecks(old); err != nil {
			t.Fatalf("seed checks: %v", err)
		}
		if _, err := s.ComputeLatencyBaseline("m1", 24*time.Hour, 10, now); err != nil {
			t.Fatalf("initial baseline: %v", err)
		}
		if err := s.RelearnLatencyBaselines(now); err != nil {
			t.Fatalf("RelearnLatencyBaselines: %v", err)
		}

		if all, err := s.GetLatencyBaselines(); err != nil || len(all) != 0 {
			t.Fatalf("baselines after reset = %v, err=%v", all, err)
		}
		var checkCount int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM monitor_checks").Scan(&checkCount); err != nil || checkCount != 20 {
			t.Fatalf("check history count = %d, err=%v; want 20", checkCount, err)
		}
		cutoff, err := s.GetSetting("notification.latency.learn_after")
		if err != nil || cutoff != now.Format(time.RFC3339Nano) {
			t.Fatalf("learn_after = %q, err=%v", cutoff, err)
		}
		if written, err := s.ComputeLatencyBaselineSince("m1", now, 10, now.Add(time.Minute)); err != nil || written {
			t.Fatalf("old checks rebuilt baseline: written=%v err=%v", written, err)
		}
	})
}
