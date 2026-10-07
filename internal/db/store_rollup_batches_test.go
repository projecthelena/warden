package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"
)

func TestRollupDailyUptime_MultipleMonitorRanges(t *testing.T) {
	RunTestWithBothDBs(t, "rollup monitor ranges", func(t *testing.T, s *Store) {
		if err := s.CreateGroup(Group{ID: "ranges", Name: "Ranges"}); err != nil {
			t.Fatal(err)
		}
		var checks []CheckResult
		for i := 0; i < 123; i++ {
			id := fmt.Sprintf("monitor-%04d", i*3)
			if err := s.CreateMonitor(Monitor{ID: id, GroupID: "ranges", Name: id, Interval: 60}); err != nil {
				t.Fatal(err)
			}
			if i%7 == 0 {
				continue
			}
			checks = append(checks, checkAt(id, "up", 1), checkAt(id, "down", 1), checkAt(id, "up", 3))
		}
		if err := s.BatchInsertChecks(checks); err != nil {
			t.Fatal(err)
		}
		for _, diagnostics := range []bool{false, true} {
			if diagnostics {
				stats, err := s.RollupDailyUptimeWithStats(2)
				if err != nil {
					t.Fatal(err)
				}
				if stats.Rows != 105 || stats.DBAfter.InUse != 0 {
					t.Fatalf("unexpected rollup stats: %+v", stats)
				}
			} else if err := s.RollupDailyUptime(2); err != nil {
				t.Fatal(err)
			}
			rows, err := s.db.Query("SELECT monitor_id, day, total, up_count FROM monitor_uptime_daily ORDER BY monitor_id")
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for rows.Next() {
				var id, day string
				var total, up int
				if err := rows.Scan(&id, &day, &total, &up); err != nil {
					t.Fatal(err)
				}
				if day != dayStr(1) || total != 2 || up != 1 {
					t.Errorf("%s: day=%s total=%d up=%d", id, day, total, up)
				}
				count++
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if err := rows.Close(); err != nil {
				t.Fatal(err)
			}
			if count != 105 {
				t.Fatalf("rollup rows = %d, want 105", count)
			}
		}
	})
}

func TestRollupDailyUptime_SQLiteRangeErrorDoesNotPublish(t *testing.T) {
	for _, diagnostics := range []bool{false, true} {
		t.Run(fmt.Sprintf("diagnostics=%t", diagnostics), func(t *testing.T) {
			s := newTestStore(t)
			t.Cleanup(func() { _ = s.Close() })
			if err := s.CreateGroup(Group{ID: "ranges", Name: "Ranges"}); err != nil {
				t.Fatal(err)
			}
			var checks []CheckResult
			for i := 0; i < 51; i++ {
				id := fmt.Sprintf("monitor-%04d", i)
				if err := s.CreateMonitor(Monitor{ID: id, GroupID: "ranges", Name: id, Interval: 60}); err != nil {
					t.Fatal(err)
				}
				checks = append(checks, checkAt(id, "up", 1))
			}
			if err := s.BatchInsertChecks(checks); err != nil {
				t.Fatal(err)
			}
			if err := s.RollupDailyUptime(2); err != nil {
				t.Fatal(err)
			}
			for i := range checks {
				checks[i].Status = "down"
			}
			if err := s.BatchInsertChecks(checks); err != nil {
				t.Fatal(err)
			}
			conn, err := s.db.Conn(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			selects := 0
			err = conn.Raw(func(raw any) error {
				raw.(*sqlite3.SQLiteConn).RegisterAuthorizer(func(op int, _, _, _ string) int {
					if op == sqlite3.SQLITE_SELECT {
						selects++
						if selects == 3 {
							return sqlite3.SQLITE_DENY
						}
					}
					return sqlite3.SQLITE_OK
				})
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := conn.Close(); err != nil {
				t.Fatal(err)
			}
			var stats DailyUptimeRollupStats
			var rollupErr error
			if diagnostics {
				stats, rollupErr = s.RollupDailyUptimeWithStats(2)
			} else {
				rollupErr = s.RollupDailyUptime(2)
			}
			conn, err = s.db.Conn(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			err = conn.Raw(func(raw any) error { raw.(*sqlite3.SQLiteConn).RegisterAuthorizer(nil); return nil })
			if err != nil {
				t.Fatal(err)
			}
			if err := conn.Close(); err != nil {
				t.Fatal(err)
			}
			if rollupErr == nil {
				t.Fatal("expected second range to fail")
			}
			if stats.DBAfter.InUse != 0 || stats.CommitDuration != 0 {
				t.Fatalf("connection leaked or partial rollup committed: %+v", stats)
			}
			var count int
			if err := s.db.QueryRow("SELECT SUM(total) FROM monitor_uptime_daily").Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 51 {
				t.Fatalf("changed previous rollup to %d checks after incomplete read", count)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := s.PingContext(ctx); err != nil {
				t.Fatal(err)
			}
			if err := s.RollupDailyUptime(2); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// A queued readiness check and writer must run before the next read batch, even
// when diagnostics are disabled. Deletion must not discard other monitors' history.
func TestRollupDailyUptime_SQLiteYieldsToQueuedWork(t *testing.T) {
	for _, diagnostics := range []bool{false, true} {
		t.Run(fmt.Sprintf("diagnostics=%t", diagnostics), func(t *testing.T) {
			s := newTestStore(t)
			t.Cleanup(func() { _ = s.Close() })
			if err := s.CreateGroup(Group{ID: "ranges", Name: "Ranges"}); err != nil {
				t.Fatal(err)
			}
			var checks []CheckResult
			for i := 0; i < 123; i++ {
				id := fmt.Sprintf("monitor-%04d", i)
				if err := s.CreateMonitor(Monitor{ID: id, GroupID: "ranges", Name: id, Interval: 60}); err != nil {
					t.Fatal(err)
				}
				checks = append(checks, checkAt(id, "up", 1))
			}
			if err := s.BatchInsertChecks(checks); err != nil {
				t.Fatal(err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			firstRead := make(chan struct{})
			releaseRead := make(chan struct{})
			defer close(releaseRead)
			writerDone := make(chan error, 1)
			conn, err := s.db.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			reads := 0
			inRead := false
			err = conn.Raw(func(raw any) error {
				raw.(*sqlite3.SQLiteConn).RegisterAuthorizer(func(op int, table, _, _ string) int {
					if op == sqlite3.SQLITE_SELECT {
						inRead = false
					}
					if op == sqlite3.SQLITE_READ && table == "monitor_checks" && !inRead {
						inRead = true
						reads++
						if reads == 1 {
							close(firstRead)
							select {
							case <-releaseRead:
							case <-ctx.Done():
								return sqlite3.SQLITE_DENY
							}
						}
					}
					return sqlite3.SQLITE_OK
				})
				return nil
			})
			_ = conn.Close()
			if err != nil {
				t.Fatal(err)
			}
			rollupDone := make(chan error, 1)
			go func() {
				if diagnostics {
					_, err := s.RollupDailyUptimeWithStats(2)
					rollupDone <- err
				} else {
					rollupDone <- s.RollupDailyUptime(2)
				}
			}()
			select {
			case <-firstRead:
			case <-ctx.Done():
				t.Fatal("rollup did not start reading checks")
			}
			before := s.Stats().WaitCount
			go func() {
				conn, err := s.db.Conn(ctx)
				if err != nil {
					writerDone <- err
					return
				}
				defer func() { _ = conn.Close() }()
				// PingContext uses the same pool as the HTTP readiness handler.
				if err := conn.PingContext(ctx); err != nil {
					writerDone <- err
					return
				}
				if reads != 1 {
					writerDone <- fmt.Errorf("queued work ran after %d read batches, want 1", reads)
					return
				}
				if _, err := conn.ExecContext(ctx, "DELETE FROM monitors WHERE id = ?", "monitor-0000"); err != nil {
					writerDone <- err
					return
				}
				_, err = conn.ExecContext(ctx, "INSERT INTO monitor_checks (monitor_id, status, latency, timestamp) VALUES (?, 'down', 0, ?)", "monitor-0001", checkAt("", "", 1).Timestamp)
				writerDone <- err
			}()
			for s.Stats().WaitCount == before {
				select {
				case <-ctx.Done():
					t.Fatal("writer did not queue behind the rollup")
				default:
					time.Sleep(time.Millisecond)
				}
			}
			releaseRead <- struct{}{}
			select {
			case err := <-writerDone:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("queued readiness/write did not complete")
			}
			select {
			case err := <-rollupDone:
				if err != nil {
					t.Fatalf("concurrent deletion aborted the rollup: %v", err)
				}
			case <-ctx.Done():
				t.Fatal("rollup did not release the connection")
			}
			if reads < 3 {
				t.Fatalf("want multiple read batches, got %d", reads)
			}
			conn, err = s.db.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			err = conn.Raw(func(raw any) error { raw.(*sqlite3.SQLiteConn).RegisterAuthorizer(nil); return nil })
			_ = conn.Close()
			if err != nil {
				t.Fatal(err)
			}
			var count int
			if err := s.db.QueryRow("SELECT COUNT(*) FROM monitor_uptime_daily").Scan(&count); err != nil || count != 122 {
				t.Fatalf("surviving history: count=%d err=%v", count, err)
			}
			// Checks written after their batch was read appear on the next refresh.
			if err := s.RollupDailyUptime(2); err != nil {
				t.Fatal(err)
			}
			var total, up int
			if err := s.db.QueryRow("SELECT total, up_count FROM monitor_uptime_daily WHERE monitor_id = ?", "monitor-0001").Scan(&total, &up); err != nil || total != 2 || up != 1 {
				t.Fatalf("next refresh did not converge: total=%d up=%d err=%v", total, up, err)
			}
		})
	}
}
