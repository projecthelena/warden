package db

import (
	"context"
	"testing"
	"time"
)

func TestRollupDiagnosticsReleasesConnectionsOnError(t *testing.T) {
	RunTestWithBothDBs(t, "rollup diagnostics failures", func(t *testing.T, s *Store) {
		if err := s.CreateGroup(Group{ID: "g1", Name: "G1"}); err != nil {
			t.Fatal(err)
		}
		if err := s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 60}); err != nil {
			t.Fatal(err)
		}
		if err := s.BatchInsertChecks([]CheckResult{checkAt("m1", "up", 1)}); err != nil {
			t.Fatal(err)
		}
		for _, table := range []string{"monitor_checks", "monitor_uptime_daily"} {
			t.Run(table, func(t *testing.T) {
				if _, err := s.db.Exec("ALTER TABLE " + table + " RENAME TO hidden_rollup_table"); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if _, err := s.db.Exec("ALTER TABLE hidden_rollup_table RENAME TO " + table); err != nil {
						t.Error(err)
					}
				}()
				stats, err := s.RollupDailyUptimeWithStats(2)
				if err == nil {
					t.Fatal("expected missing-table error")
				}
				if stats.DBAfter.InUse != 0 || stats.CommitDuration != 0 {
					t.Fatalf("connection leaked or failed rollup committed: %+v", stats)
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				if err := s.db.PingContext(ctx); err != nil {
					t.Fatalf("pool unavailable after error: %v", err)
				}
			})
		}
		if err := s.RollupDailyUptime(2); err != nil {
			t.Fatalf("rollup did not recover: %v", err)
		}
	})
}

func TestRollupDiagnosticsSeparatesConnectionWait(t *testing.T) {
	s := newTestStore(t)
	t.Cleanup(func() { _ = s.Close() })
	conn, err := s.db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	before := s.Stats()
	type result struct {
		stats DailyUptimeRollupStats
		err   error
	}
	done := make(chan result, 1)
	go func() {
		stats, err := s.RollupDailyUptimeWithStats(2)
		done <- result{stats, err}
	}()
	deadline := time.Now().Add(time.Second)
	for s.Stats().WaitCount == before.WaitCount {
		if time.Now().After(deadline) {
			t.Fatal("rollup did not wait for occupied connection")
		}
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	_ = conn.Close()
	select {
	case got := <-done:
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.stats.ReadConnectionDuration < 20*time.Millisecond || got.stats.DBAfter.WaitCount <= got.stats.DBBefore.WaitCount {
			t.Fatalf("missing connection contention: %+v", got.stats)
		}
		if got.stats.Rows != 0 || got.stats.BeginDuration != 0 || got.stats.DBAfter.InUse != 0 {
			t.Fatalf("empty rollup should release its read connection without writing: %+v", got.stats)
		}
	case <-time.After(time.Second):
		t.Fatal("rollup did not finish after connection was returned")
	}
}
