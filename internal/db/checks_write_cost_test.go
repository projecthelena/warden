package db

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewIngestID_SortsByCreationTime(t *testing.T) {
	now := time.Now()
	earlier, later := newIngestID(now), newIngestID(now.Add(time.Microsecond))
	if len(earlier) != 26 || earlier >= later {
		t.Fatalf("ingest IDs %q and %q are not fixed-width and time-ordered", earlier, later)
	}
}

func TestCheckPagination_TiedTimestamps(t *testing.T) {
	RunTestWithBothDBs(t, "tied pagination", func(t *testing.T, s *Store) {
		if err := s.CreateGroup(Group{ID: "g-page", Name: "Page"}); err != nil {
			t.Fatal(err)
		}
		if err := s.CreateMonitor(Monitor{ID: "m-page", GroupID: "g-page", Name: "Page", URL: "https://example.test", Interval: 60}); err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC().Truncate(time.Second)
		var checks []CheckResult
		for i := 0; i < 5; i++ {
			checks = append(checks, CheckResult{MonitorID: "m-page", Timestamp: now, Status: "up", Latency: int64(i)})
		}
		checks = append(checks, CheckResult{MonitorID: "m-page", Timestamp: now.Add(-time.Minute), Status: "down"})
		if err := s.BatchInsertChecks(checks); err != nil {
			t.Fatal(err)
		}
		seen := map[int64]bool{}
		var before int64
		var order []CheckResult
		for {
			page, err := s.GetMonitorChecksPage("m-page", 2, before)
			if err != nil {
				t.Fatal(err)
			}
			if len(page) == 0 {
				break
			}
			for _, c := range page {
				if seen[c.ID] {
					t.Fatalf("check %d returned twice", c.ID)
				}
				seen[c.ID] = true
				order = append(order, c)
			}
			before = page[len(page)-1].ID
		}
		if len(order) != 6 || order[5].Status != "down" {
			t.Fatalf("pages = %+v", order)
		}
		for i := 1; i < 5; i++ {
			if order[i].ID >= order[i-1].ID {
				t.Fatalf("tied checks not ordered by id: %+v", order)
			}
		}
	})
}

func TestCheckBatch_AsyncCommitIsScopedToTransaction(t *testing.T) {
	RunTestWithBothDBs(t, "async check commit", func(t *testing.T, s *Store) {
		if !s.IsPostgres() {
			t.Skip("synchronous_commit only exists on PostgreSQL")
		}
		if err := s.CreateGroup(Group{ID: "g-async", Name: "Async"}); err != nil {
			t.Fatal(err)
		}
		if err := s.CreateMonitor(Monitor{ID: "m-async", GroupID: "g-async", Name: "Async", URL: "https://example.test", Interval: 60}); err != nil {
			t.Fatal(err)
		}
		ctx := context.Background()
		conn, err := s.db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = conn.Close() }()
		var before, inside, after string
		if err := conn.QueryRowContext(ctx, "SHOW synchronous_commit").Scan(&before); err != nil {
			t.Fatal(err)
		}
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.insertChecks(ctx, tx, []CheckResult{{MonitorID: "m-async", Timestamp: time.Now().UTC(), Status: "up"}}); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRowContext(ctx, "SHOW synchronous_commit").Scan(&inside); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRowContext(ctx, "SHOW synchronous_commit").Scan(&after); err != nil {
			t.Fatal(err)
		}
		if inside != "off" || after != before {
			t.Fatalf("synchronous_commit before = %q, inside = %q, after = %q", before, inside, after)
		}
	})
}

func TestDropCheckIDPaginationIndexMigration_RoundTrip(t *testing.T) {
	RunTestWithBothDBs(t, "drop check id index", func(t *testing.T, s *Store) {
		exists := func() bool {
			query := "SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_monitor_checks_monitor_id_id'"
			if s.IsPostgres() {
				query = "SELECT COUNT(*) FROM pg_indexes WHERE indexname = 'idx_monitor_checks_monitor_id_id'"
			}
			var n int
			if err := s.db.QueryRow(query).Scan(&n); err != nil {
				t.Fatal(err)
			}
			return n == 1
		}
		if exists() {
			t.Fatal("index still present after migrations")
		}
		migrationFS, path := sqliteMigrationFS, "migrations/sqlite/00039_drop_check_id_pagination_index.sql"
		if s.IsPostgres() {
			migrationFS, path = postgresMigrationFS, "migrations/postgres/00039_drop_check_id_pagination_index.sql"
		}
		script, err := migrationFS.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.Split(string(script), "-- +goose Down")
		if _, err := s.db.Exec(parts[1]); err != nil || !exists() {
			t.Fatalf("down: exists = %v, err = %v", exists(), err)
		}
		if _, err := s.db.Exec(parts[0]); err != nil || exists() {
			t.Fatalf("up: exists = %v, err = %v", exists(), err)
		}
	})
}
