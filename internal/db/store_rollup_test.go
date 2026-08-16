package db

import (
	"testing"
	"time"
)

// dayStr is the UTC calendar day N days ago, as stored in the rollup.
func dayStr(daysAgo int) string {
	return time.Now().UTC().AddDate(0, 0, -daysAgo).Format("2006-01-02")
}

// checkAt builds a check stamped at noon UTC on the given day, well inside any day boundary.
func checkAt(monitorID, status string, daysAgo int) CheckResult {
	d := time.Now().UTC().AddDate(0, 0, -daysAgo)
	ts := time.Date(d.Year(), d.Month(), d.Day(), 12, 0, 0, 0, time.UTC)
	return CheckResult{MonitorID: monitorID, Status: status, Timestamp: ts}
}

func findDay(stats []DailyUptimeStat, day string) (DailyUptimeStat, bool) {
	for _, s := range stats {
		if s.Date == day {
			return s, true
		}
	}
	return DailyUptimeStat{}, false
}

func TestRollupDailyUptime_MatchesLiveAndEdges(t *testing.T) {
	RunTestWithBothDBs(t, "rollup daily uptime", func(t *testing.T, s *Store) {
		_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})
		_ = s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 60})
		_ = s.CreateMonitor(Monitor{ID: "m2", GroupID: "g1", Name: "M2", Interval: 60})

		checks := []CheckResult{
			// m1: day-3 all up (2/2), day-1 mixed (1 up 1 down), today all up (2/2). day-2 gap.
			checkAt("m1", "up", 3), checkAt("m1", "up", 3),
			checkAt("m1", "up", 1), checkAt("m1", "down", 1),
			checkAt("m1", "up", 0), checkAt("m1", "up", 0),
			// m2: day-1 all down (0/2). No other days.
			checkAt("m2", "down", 1), checkAt("m2", "down", 1),
		}
		if err := s.BatchInsertChecks(checks); err != nil {
			t.Fatalf("BatchInsertChecks: %v", err)
		}

		if err := s.RollupDailyUptime(10); err != nil {
			t.Fatalf("RollupDailyUptime: %v", err)
		}

		got, err := s.GetDailyUptimeStatsForMonitors([]string{"m1", "m2"}, 10)
		if err != nil {
			t.Fatalf("GetDailyUptimeStatsForMonitors: %v", err)
		}

		// Equivalence: the rollup read matches the live per-monitor aggregation day for day.
		for _, id := range []string{"m1", "m2"} {
			live, err := s.GetDailyUptimeStats(id, 10)
			if err != nil {
				t.Fatalf("GetDailyUptimeStats(%s): %v", id, err)
			}
			if len(live) != len(got[id]) {
				t.Fatalf("%s: len live=%d rollup=%d", id, len(live), len(got[id]))
			}
			for i := range live {
				if live[i] != got[id][i] {
					t.Errorf("%s day %s: live=%+v rollup=%+v", id, live[i].Date, live[i], got[id][i])
				}
			}
		}

		// A mixed day is a real percentage.
		if d, ok := findDay(got["m1"], dayStr(1)); !ok || d.UptimePercent != 50 || d.Total != 2 {
			t.Errorf("m1 day-1 want 50%% of 2, got %+v (ok=%v)", d, ok)
		}
		// An all-down day is 0%, not no-data: it has checks.
		if d, ok := findDay(got["m2"], dayStr(1)); !ok || d.UptimePercent != 0 || d.Total != 2 {
			t.Errorf("m2 day-1 want 0%% of 2, got %+v (ok=%v)", d, ok)
		}
		// A day with no checks is no-data (-1), distinct from 0%.
		if d, ok := findDay(got["m1"], dayStr(2)); !ok || d.UptimePercent != -1 || d.Total != 0 {
			t.Errorf("m1 day-2 (gap) want -1/no data, got %+v", d)
		}
		// A monitor with no checks on a day it didn't run is no-data too.
		if d, ok := findDay(got["m2"], dayStr(3)); !ok || d.UptimePercent != -1 {
			t.Errorf("m2 day-3 want -1/no data, got %+v", d)
		}
	})
}

func TestRollupDailyUptime_Idempotent(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})
	_ = s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 60})
	_ = s.BatchInsertChecks([]CheckResult{checkAt("m1", "up", 1), checkAt("m1", "down", 1)})

	if err := s.RollupDailyUptime(5); err != nil {
		t.Fatalf("rollup 1: %v", err)
	}
	if err := s.RollupDailyUptime(5); err != nil {
		t.Fatalf("rollup 2: %v", err)
	}
	got, _ := s.GetDailyUptimeStatsForMonitors([]string{"m1"}, 5)
	if d, ok := findDay(got["m1"], dayStr(1)); !ok || d.Total != 2 || d.UptimePercent != 50 {
		t.Errorf("after double rollup want 50%% of 2, got %+v", d)
	}

	// A late check for the same day is picked up on the next recompute (upsert overwrites).
	_ = s.BatchInsertChecks([]CheckResult{checkAt("m1", "up", 1)})
	if err := s.RollupDailyUptime(5); err != nil {
		t.Fatalf("rollup 3: %v", err)
	}
	got, _ = s.GetDailyUptimeStatsForMonitors([]string{"m1"}, 5)
	if d, _ := findDay(got["m1"], dayStr(1)); d.Total != 3 || d.Up != 2 {
		t.Errorf("late check not folded in: want 2/3, got %+v", d)
	}
}

// The whole point of the rollup: an old bar survives even after its raw checks are pruned,
// and recomputing only recent days does not touch it.
func TestRollupDailyUptime_FrozenAfterCheckPrune(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})
	_ = s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 60})
	_ = s.BatchInsertChecks([]CheckResult{
		checkAt("m1", "up", 5), checkAt("m1", "down", 5), // day-5: 50%
		checkAt("m1", "up", 0), // today
	})
	if err := s.RollupDailyUptime(10); err != nil {
		t.Fatalf("rollup: %v", err)
	}

	// Prune checks older than 3 days (day-5 checks are gone), then recompute only recent days.
	if err := s.PruneMonitorChecks(3); err != nil {
		t.Fatalf("prune checks: %v", err)
	}
	if err := s.RollupDailyUptime(2); err != nil {
		t.Fatalf("recompute recent: %v", err)
	}

	got, _ := s.GetDailyUptimeStatsForMonitors([]string{"m1"}, 10)
	if d, ok := findDay(got["m1"], dayStr(5)); !ok || d.UptimePercent != 50 || d.Total != 2 {
		t.Errorf("day-5 bar should survive check pruning frozen at 50%%, got %+v (ok=%v)", d, ok)
	}
}

// A 365-day range is the case that most benefits from the rollup (it used to aggregate a
// year of raw checks per request). It must read a full 365-entry series from the rollup.
func TestRollupDailyUptime_LargeRange(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})
	_ = s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 60})
	_ = s.BatchInsertChecks([]CheckResult{
		checkAt("m1", "up", 0),   // today
		checkAt("m1", "up", 300), // ~10 months ago, still inside 365
		checkAt("m1", "down", 300),
	})
	if err := s.RollupDailyUptime(365); err != nil {
		t.Fatalf("rollup: %v", err)
	}

	got, err := s.GetDailyUptimeStatsForMonitors([]string{"m1"}, 365)
	if err != nil {
		t.Fatalf("read 365: %v", err)
	}
	if len(got["m1"]) != 365 {
		t.Fatalf("expected 365 entries, got %d", len(got["m1"]))
	}
	if d, ok := findDay(got["m1"], dayStr(300)); !ok || d.UptimePercent != 50 || d.Total != 2 {
		t.Errorf("day-300 want 50%% of 2, got %+v", d)
	}
	if d, ok := findDay(got["m1"], dayStr(0)); !ok || d.UptimePercent != 100 {
		t.Errorf("today want 100%%, got %+v", d)
	}
	if d, _ := findDay(got["m1"], dayStr(150)); d.UptimePercent != -1 {
		t.Errorf("day-150 (no checks) want -1, got %+v", d)
	}
}

// Deleting a monitor must take its rollup rows with it (FK ON DELETE CASCADE), so no
// orphans accumulate and a reused id never inherits stale bars.
func TestRollupDailyUptime_CascadesOnMonitorDelete(t *testing.T) {
	RunTestWithBothDBs(t, "rollup cascade", func(t *testing.T, s *Store) {
		_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})
		_ = s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 60})
		_ = s.BatchInsertChecks([]CheckResult{checkAt("m1", "up", 1)})
		if err := s.RollupDailyUptime(5); err != nil {
			t.Fatalf("rollup: %v", err)
		}

		var before int
		_ = s.db.QueryRow(s.rebind("SELECT COUNT(*) FROM monitor_uptime_daily WHERE monitor_id = ?"), "m1").Scan(&before)
		if before == 0 {
			t.Fatal("expected a rollup row before delete")
		}

		if err := s.DeleteMonitor("m1"); err != nil {
			t.Fatalf("delete: %v", err)
		}

		var after int
		_ = s.db.QueryRow(s.rebind("SELECT COUNT(*) FROM monitor_uptime_daily WHERE monitor_id = ?"), "m1").Scan(&after)
		if after != 0 {
			t.Errorf("rollup rows should be gone after monitor delete, got %d", after)
		}
	})
}

func TestRollupDailyUptime_NoChecks(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})
	_ = s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 60})

	// No checks at all: rollup is a no-op, no error, and the read is all no-data.
	if err := s.RollupDailyUptime(30); err != nil {
		t.Fatalf("rollup with no checks: %v", err)
	}
	got, _ := s.GetDailyUptimeStatsForMonitors([]string{"m1"}, 7)
	for _, d := range got["m1"] {
		if d.UptimePercent != -1 {
			t.Errorf("expected all no-data, got %+v", d)
		}
	}
}

func TestPruneDailyRollups(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateGroup(Group{ID: "g1", Name: "G1"})
	_ = s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 60})
	_ = s.BatchInsertChecks([]CheckResult{checkAt("m1", "up", 40), checkAt("m1", "up", 1)})
	if err := s.RollupDailyUptime(90); err != nil {
		t.Fatalf("rollup: %v", err)
	}

	if err := s.PruneDailyRollups(30); err != nil {
		t.Fatalf("prune: %v", err)
	}
	got, _ := s.GetDailyUptimeStatsForMonitors([]string{"m1"}, 90)
	if d, _ := findDay(got["m1"], dayStr(40)); d.UptimePercent != -1 {
		t.Errorf("day-40 should be pruned (no data), got %+v", d)
	}
	if d, ok := findDay(got["m1"], dayStr(1)); !ok || d.UptimePercent != 100 {
		t.Errorf("day-1 should survive prune, got %+v", d)
	}

	if err := s.PruneDailyRollups(0); err == nil {
		t.Error("expected error for 0 days")
	}
}
