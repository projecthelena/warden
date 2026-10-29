package uptime

import (
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

// seedRetentionHistory stores checks at noon UTC 30 days ago and yesterday, rolled up.
func seedRetentionHistory(t *testing.T, store *db.Store) (oldDay, recentDay string) {
	t.Helper()
	if err := store.CreateGroup(db.Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateMonitor(db.Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 60}); err != nil {
		t.Fatal(err)
	}
	noon := func(daysAgo int) time.Time {
		d := time.Now().UTC().AddDate(0, 0, -daysAgo)
		return time.Date(d.Year(), d.Month(), d.Day(), 12, 0, 0, 0, time.UTC)
	}
	if err := store.BatchInsertChecks([]db.CheckResult{
		{MonitorID: "m1", Status: "up", Timestamp: noon(30)},
		{MonitorID: "m1", Status: "down", Timestamp: noon(30)},
		{MonitorID: "m1", Status: "up", Timestamp: noon(1)},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.RollupDailyUptime(60); err != nil {
		t.Fatal(err)
	}
	return noon(30).Format("2006-01-02"), noon(1).Format("2006-01-02")
}

func rollupTotal(t *testing.T, store *db.Store, day string) int {
	t.Helper()
	stats, err := store.GetDailyUptimeStatsForMonitors([]string{"m1"}, 60)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range stats["m1"] {
		if s.Date == day {
			return s.Total
		}
	}
	t.Fatalf("day %s missing from rollup read", day)
	return 0
}

func newRetentionStore(t *testing.T) *db.Store {
	t.Helper()
	// SQLite only: these tests rewrite global settings, and a PostgreSQL database is
	// shared with other packages running in parallel. Store-level PostgreSQL coverage
	// lives in internal/db (TestRetentionDays_SplitPrune).
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestPruneHistory_RollupsOutliveChecks(t *testing.T) {
	store := newRetentionStore(t)
	oldDay, recentDay := seedRetentionHistory(t, store)
	if err := store.SetSetting("data_retention_days", "7"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSetting("rollup_retention_days", "365"); err != nil {
		t.Fatal(err)
	}

	NewManager(store).pruneHistory()

	checks, err := store.GetMonitorChecks("m1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 1 {
		t.Fatalf("checks after prune = %d, want only yesterday's", len(checks))
	}
	if got := rollupTotal(t, store, oldDay); got != 2 {
		t.Fatalf("30-day-old rollup total = %d, want 2 kept past check retention", got)
	}
	if got := rollupTotal(t, store, recentDay); got != 1 {
		t.Fatalf("recent rollup total = %d, want 1", got)
	}
}

// Without rollup_retention_days, rollups are pruned with the checks, exactly as before.
func TestPruneHistory_RollupsFollowCheckRetentionByDefault(t *testing.T) {
	store := newRetentionStore(t)
	oldDay, recentDay := seedRetentionHistory(t, store)
	if err := store.SetSetting("data_retention_days", "7"); err != nil {
		t.Fatal(err)
	}

	NewManager(store).pruneHistory()

	if got := rollupTotal(t, store, oldDay); got != 0 {
		t.Fatalf("30-day-old rollup total = %d, want pruned with data_retention_days", got)
	}
	if got := rollupTotal(t, store, recentDay); got != 1 {
		t.Fatalf("recent rollup total = %d, want 1", got)
	}
}
