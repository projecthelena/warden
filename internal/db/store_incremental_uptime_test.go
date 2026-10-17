package db

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func incrementalMonitor(t *testing.T, s *Store) {
	t.Helper()
	s.incrementalUptime = true
	if err := s.configureUptime(); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateGroup(Group{ID: "g1", Name: "G"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M", Interval: 10}); err != nil {
		t.Fatal(err)
	}
}

func assertIncrementalCounts(t *testing.T, s *Store, day string, total, up int) {
	t.Helper()
	var gotTotal, gotUp int
	if err := s.db.QueryRow(s.rebind("SELECT total, up_count FROM monitor_uptime_daily WHERE monitor_id = ? AND day = ?"), "m1", day).Scan(&gotTotal, &gotUp); err != nil {
		t.Fatal(err)
	}
	if gotTotal != total || gotUp != up {
		t.Fatalf("%s: got %d/%d up, want %d/%d", day, gotUp, gotTotal, up, total)
	}
}

func TestIncrementalUptimeRetriesAndUTC(t *testing.T) {
	RunTestWithBothDBs(t, "incremental retries", func(t *testing.T, s *Store) {
		incrementalMonitor(t, s)
		stamp := time.Date(2026, 10, 16, 23, 59, 0, 0, time.FixedZone("west", -5*3600))
		batch := []CheckResult{{MonitorID: "m1", Status: "up", Timestamp: stamp}, {MonitorID: "m1", Status: "down", Timestamp: stamp.Add(-24 * time.Hour)}}
		if err := s.BatchInsertChecks(batch); err != nil {
			t.Fatal(err)
		}
		if batch[0].IngestID == "" {
			t.Fatal("identity not retained for retry")
		}
		// This is also the retry path after a successful commit with a lost response.
		if err := s.BatchInsertChecks(batch); err != nil {
			t.Fatal(err)
		}
		mixed := append(append([]CheckResult{}, batch...), CheckResult{MonitorID: "m1", Status: "up", Timestamp: stamp})
		if err := s.BatchInsertChecks(mixed); err != nil {
			t.Fatal(err)
		}
		assertIncrementalCounts(t, s, "2026-10-17", 2, 2)
		assertIncrementalCounts(t, s, "2026-10-16", 1, 0)
		var n int
		if err := s.db.QueryRow("SELECT count(*) FROM monitor_checks").Scan(&n); err != nil || n != 3 {
			t.Fatalf("raw checks=%d err=%v", n, err)
		}
	})
}

func TestIncrementalUptimeRollbackAndDeletedMonitor(t *testing.T) {
	RunTestWithBothDBs(t, "atomic rollback", func(t *testing.T, s *Store) {
		incrementalMonitor(t, s)
		batch := []CheckResult{checkAt("m1", "up", 0), checkAt("deleted", "down", 0)}
		// Force failure after the check INSERT, at the aggregate write.
		if _, err := s.db.Exec("ALTER TABLE monitor_uptime_daily RENAME TO saved_uptime_daily"); err != nil {
			t.Fatal(err)
		}
		if err := s.BatchInsertChecks(batch); err == nil {
			t.Fatal("expected aggregate write failure")
		}
		var n int
		if err := s.db.QueryRow("SELECT count(*) FROM monitor_checks").Scan(&n); err != nil || n != 0 {
			t.Fatalf("rollback raw=%d err=%v", n, err)
		}
		if _, err := s.db.Exec("ALTER TABLE saved_uptime_daily RENAME TO monitor_uptime_daily"); err != nil {
			t.Fatal(err)
		}
		if err := s.BatchInsertChecks(batch); err != nil {
			t.Fatal(err)
		}
		assertIncrementalCounts(t, s, dayStr(0), 1, 1)
		// Retention of raw data never decrements the historical summary.
		if _, err := s.db.Exec("DELETE FROM monitor_checks"); err != nil {
			t.Fatal(err)
		}
		assertIncrementalCounts(t, s, dayStr(0), 1, 1)
	})
}

func TestIncrementalUptimeConcurrentBatches(t *testing.T) {
	RunTestWithBothDBs(t, "concurrent increments", func(t *testing.T, s *Store) {
		incrementalMonitor(t, s)
		const writers = 8
		errors := make(chan error, writers)
		var wg sync.WaitGroup
		for i := 0; i < writers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				batch := []CheckResult{checkAt("m1", "up", 0), checkAt("m1", "down", 0)}
				err := s.BatchInsertChecks(batch)
				if err == nil {
					err = s.BatchInsertChecks(batch)
				}
				errors <- err
			}()
		}
		wg.Wait()
		close(errors)
		for err := range errors {
			if err != nil {
				t.Fatal(err)
			}
		}
		assertIncrementalCounts(t, s, dayStr(0), writers*2, writers)
	})
}

func TestIncrementalUptimeCancelledWrite(t *testing.T) {
	RunTestWithBothDBs(t, "cancelled persistence", func(t *testing.T, s *Store) {
		incrementalMonitor(t, s)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		batch := []CheckResult{checkAt("m1", "up", 0)}
		if err := s.BatchInsertChecksContext(ctx, batch); err == nil {
			t.Fatal("cancelled write succeeded")
		}
		if err := s.BatchInsertChecks(batch); err != nil {
			t.Fatal(err)
		}
		assertIncrementalCounts(t, s, dayStr(0), 1, 1)
	})
}

func TestIncrementalUptimeMigration(t *testing.T) {
	RunTestWithBothDBs(t, "incremental migration", func(t *testing.T, s *Store) {
		incrementalMonitor(t, s)
		fs, path := sqliteMigrationFS, "migrations/sqlite/00037_incremental_uptime.sql"
		if s.IsPostgres() {
			fs, path = postgresMigrationFS, "migrations/postgres/00037_incremental_uptime.sql"
		}
		data, err := fs.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.Split(string(data), "-- +goose Down")
		exec := func(query string) {
			t.Helper()
			tx, err := s.db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback() }()
			if _, err = tx.Exec(query); err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(); err != nil {
				t.Fatal(err)
			}
		}
		exec(parts[1])
		if _, err = s.db.Exec(s.rebind("INSERT INTO settings (key,value) VALUES (?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value"), "data_retention_days", "2"); err != nil {
			t.Fatal(err)
		}
		for _, days := range []int{0, 1, 2} {
			c := checkAt("m1", "up", days)
			if _, err = s.db.Exec(s.rebind("INSERT INTO monitor_checks (monitor_id,status,timestamp,latency) VALUES (?,?,?,0)"), c.MonitorID, c.Status, c.Timestamp); err != nil {
				t.Fatal(err)
			}
		}
		for _, days := range []int{0, 2, 3} {
			if _, err = s.db.Exec(s.rebind("INSERT INTO monitor_uptime_daily (monitor_id,day,total,up_count) VALUES (?,?,?,?)"), "m1", dayStr(days), 10, 7); err != nil {
				t.Fatal(err)
			}
		}
		// A failed initialization transaction leaves neither schema nor partial counts.
		tx, err := s.db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(parts[0]); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
		if err = tx.Rollback(); err != nil {
			t.Fatal(err)
		}
		assertIncrementalCounts(t, s, dayStr(0), 10, 7)
		if _, err = s.db.Exec("SELECT ingest_id FROM monitor_checks"); err == nil {
			t.Fatal("schema change survived rollback")
		}
		exec(parts[0])
		if err := s.configureUptime(); err != nil {
			t.Fatal(err)
		}
		assertIncrementalCounts(t, s, dayStr(0), 1, 1)
		assertIncrementalCounts(t, s, dayStr(1), 1, 1)
		assertIncrementalCounts(t, s, dayStr(2), 10, 7)
		assertIncrementalCounts(t, s, dayStr(3), 10, 7)
		if err = s.BatchInsertChecks([]CheckResult{checkAt("m1", "down", 0)}); err != nil {
			t.Fatal(err)
		}
		// Re-running Goose after restart does not repeat the initialization.
		if err = s.migrate(); err != nil {
			t.Fatal(err)
		}
		assertIncrementalCounts(t, s, dayStr(0), 2, 1)
		t.Logf("%s: migration rollback, retention boundary and restart verified", s.Dialect())
	})
}

func TestIncrementalUptimePruneAndRestart(t *testing.T) {
	RunTestWithBothDBs(t, "retention and restart", func(t *testing.T, s *Store) {
		incrementalMonitor(t, s)
		if err := s.BatchInsertChecks([]CheckResult{checkAt("m1", "up", 10), checkAt("m1", "down", 0)}); err != nil {
			t.Fatal(err)
		}
		if err := s.PruneMonitorChecks(1); err != nil {
			t.Fatal(err)
		}
		assertIncrementalCounts(t, s, dayStr(10), 1, 1)
		assertIncrementalCounts(t, s, dayStr(0), 1, 0)
		if err := s.migrate(); err != nil {
			t.Fatal(err)
		}
		assertIncrementalCounts(t, s, dayStr(10), 1, 1)
		if err := s.DeleteMonitor("m1"); err != nil {
			t.Fatal(err)
		}
		if err := s.BatchInsertChecks([]CheckResult{checkAt("m1", "up", 0)}); err != nil {
			t.Fatal(err)
		}
		var n int
		if err := s.db.QueryRow("SELECT count(*) FROM monitor_uptime_daily").Scan(&n); err != nil || n != 0 {
			t.Fatalf("deleted monitor summary=%d err=%v", n, err)
		}
	})
}

func TestIncrementalUptimeLargeBatch(t *testing.T) {
	RunTestWithBothDBs(t, "parameter boundaries", func(t *testing.T, s *Store) {
		incrementalMonitor(t, s)
		batch := make([]CheckResult, 250)
		for i := range batch {
			id := fmt.Sprintf("batch-%03d", i)
			if err := s.CreateMonitor(Monitor{ID: id, GroupID: "g1", Name: id, Interval: 10}); err != nil {
				t.Fatal(err)
			}
			batch[i] = checkAt(id, "up", 0)
		}
		if err := s.BatchInsertChecks(batch); err != nil {
			t.Fatal(err)
		}
		if err := s.BatchInsertChecks(batch); err != nil {
			t.Fatal(err)
		}
		var groups, total, up int
		if err := s.db.QueryRow("SELECT count(*),sum(total),sum(up_count) FROM monitor_uptime_daily").Scan(&groups, &total, &up); err != nil {
			t.Fatal(err)
		}
		if groups != 250 || total != 250 || up != 250 {
			t.Fatalf("groups=%d total=%d up=%d", groups, total, up)
		}
	})
}

func TestIncrementalUptimeConcurrentReset(t *testing.T) {
	RunTestWithBothDBs(t, "admin reset", func(t *testing.T, s *Store) {
		incrementalMonitor(t, s)
		started := make(chan struct{})
		done := make(chan error, 1)
		go func() {
			close(started)
			for i := 0; i < 20; i++ {
				if err := s.BatchInsertChecks([]CheckResult{checkAt("m1", "up", 0)}); err != nil {
					done <- err
					return
				}
			}
			done <- nil
		}()
		<-started
		if err := s.Reset(); err != nil {
			t.Fatal(err)
		}
		if err := <-done; err != nil {
			t.Fatalf("persistence failed during reset: %v", err)
		}
		incrementalMonitor(t, s)
		if err := s.BatchInsertChecks([]CheckResult{checkAt("m1", "up", 0)}); err != nil {
			t.Fatal(err)
		}
		assertIncrementalCounts(t, s, dayStr(0), 1, 1)
	})
}
