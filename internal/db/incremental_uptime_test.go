package db

import (
	"path/filepath"
	"testing"
)

func TestIncrementalUptimeFeatureFlagCycle(t *testing.T) {
	for _, config := range GetTestConfigs(t) {
		t.Run(config.Name, func(t *testing.T) {
			cfg := config.Config
			if cfg.Type == DialectSQLite {
				cfg.Path = filepath.Join(t.TempDir(), "toggle.sqlite")
			}
			s, err := NewStore(cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = s.Reset(); _ = s.Close() }()
			if err = s.Reset(); err != nil {
				t.Fatal(err)
			}
			if err = s.CreateGroup(Group{ID: "g1", Name: "G"}); err != nil {
				t.Fatal(err)
			}
			if err = s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M", Interval: 10}); err != nil {
				t.Fatal(err)
			}
			reopen := func(enabled bool) {
				t.Helper()
				if err := s.Close(); err != nil {
					t.Fatal(err)
				}
				cfg.IncrementalUptime = enabled
				next, err := NewStore(cfg)
				if err != nil {
					t.Fatal(err)
				}
				s = next
				if s.IncrementalUptimeEnabled() != enabled {
					t.Fatal("mode not applied")
				}
			}
			if s.IncrementalUptimeEnabled() {
				t.Fatal("feature must default off")
			}
			if err = s.BatchInsertChecks([]CheckResult{checkAt("m1", "up", 0)}); err != nil {
				t.Fatal(err)
			}
			var n int
			if err = s.db.QueryRow("SELECT count(*) FROM monitor_uptime_daily").Scan(&n); err != nil || n != 0 {
				t.Fatalf("legacy write changed summary: %d %v", n, err)
			}
			reopen(true)
			assertIncrementalCounts(t, s, dayStr(0), 1, 1)
			retry := []CheckResult{checkAt("m1", "down", 0)}
			if err = s.BatchInsertChecks(retry); err != nil {
				t.Fatal(err)
			}
			reopen(true)
			if err = s.BatchInsertChecks(retry); err != nil {
				t.Fatal(err)
			}
			assertIncrementalCounts(t, s, dayStr(0), 2, 1)
			reopen(false)
			if err = s.BatchInsertChecks([]CheckResult{checkAt("m1", "up", 0)}); err != nil {
				t.Fatal(err)
			}
			assertIncrementalCounts(t, s, dayStr(0), 2, 1)
			// Legacy refresh is available again with the feature disabled.
			if err = s.RollupDailyUptime(2); err != nil {
				t.Fatal(err)
			}
			assertIncrementalCounts(t, s, dayStr(0), 3, 2)
			reopen(true)
			assertIncrementalCounts(t, s, dayStr(0), 3, 2)
			if err = s.BatchInsertChecks(retry); err != nil {
				t.Fatal(err)
			}
			assertIncrementalCounts(t, s, dayStr(0), 3, 2)
		})
	}
}

func TestIncrementalUptimeFailedInitializationIsRetryable(t *testing.T) {
	RunTestWithBothDBs(t, "initialization rollback", func(t *testing.T, s *Store) {
		if err := s.CreateGroup(Group{ID: "g1", Name: "G"}); err != nil {
			t.Fatal(err)
		}
		if err := s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M", Interval: 10}); err != nil {
			t.Fatal(err)
		}
		if err := s.BatchInsertChecks([]CheckResult{checkAt("m1", "up", 0)}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.Exec("ALTER TABLE monitor_uptime_daily RENAME TO held_daily"); err != nil {
			t.Fatal(err)
		}
		s.incrementalUptime = true
		if err := s.configureUptime(); err == nil {
			t.Fatal("initialization should fail")
		}
		var n int
		if err := s.db.QueryRow(s.rebind("SELECT count(*) FROM settings WHERE key = ?"), incrementalUptimeMarker).Scan(&n); err != nil || n != 0 {
			t.Fatalf("failed initialization marked complete: %d %v", n, err)
		}
		if _, err := s.db.Exec("ALTER TABLE held_daily RENAME TO monitor_uptime_daily"); err != nil {
			t.Fatal(err)
		}
		if err := s.configureUptime(); err != nil {
			t.Fatal(err)
		}
		assertIncrementalCounts(t, s, dayStr(0), 1, 1)
	})
}

func TestIncrementalUptimeInitializedSkipsRawScan(t *testing.T) {
	RunTestWithBothDBs(t, "skip repeated backfill", func(t *testing.T, s *Store) {
		incrementalMonitor(t, s)
		if err := s.BatchInsertChecks([]CheckResult{checkAt("m1", "up", 0)}); err != nil {
			t.Fatal(err)
		}
		// An initialized startup must not touch the raw table, even if reading it fails.
		if _, err := s.db.Exec("DROP TABLE monitor_checks"); err != nil {
			t.Fatal(err)
		}
		if err := s.configureUptime(); err != nil {
			t.Fatal(err)
		}
		assertIncrementalCounts(t, s, dayStr(0), 1, 1)
	})
}
