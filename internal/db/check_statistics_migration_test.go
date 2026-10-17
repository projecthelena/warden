package db

import (
	"strings"
	"testing"
)

func TestCheckStatisticsMigration_RoundTrip(t *testing.T) {
	RunTestWithBothDBs(t, "check statistics migration", func(t *testing.T, s *Store) {
		if err := s.CreateGroup(Group{ID: "g1", Name: "G1"}); err != nil {
			t.Fatal(err)
		}
		if err := s.CreateMonitor(Monitor{ID: "m1", GroupID: "g1", Name: "M1", Interval: 60}); err != nil {
			t.Fatal(err)
		}
		if err := s.BatchInsertChecks([]CheckResult{checkAt("m1", "up", 1), checkAt("m1", "down", 1)}); err != nil {
			t.Fatal(err)
		}
		migrationFS, path := sqliteMigrationFS, "migrations/sqlite/00036_monitor_check_statistics_index.sql"
		if s.IsPostgres() {
			migrationFS, path = postgresMigrationFS, "migrations/postgres/00036_monitor_check_statistics_index.sql"
		}
		script, err := migrationFS.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.Split(string(script), "-- +goose Down")
		for _, step := range []struct{ sql, index string }{
			{parts[1], "idx_monitor_checks_monitor_id_ts"},
			{parts[0], "idx_monitor_checks_statistics"},
		} {
			if _, err := s.db.Exec(step.sql); err != nil {
				t.Fatal(err)
			}
			if s.IsPostgres() {
				var exists bool
				if err := s.db.QueryRow("SELECT to_regclass($1) IS NOT NULL", step.index).Scan(&exists); err != nil || !exists {
					t.Fatalf("index %s exists = %v, err = %v", step.index, exists, err)
				}
			}
			if err := s.RollupDailyUptime(2); err != nil {
				t.Fatal(err)
			}
			got, err := s.GetDailyUptimeStatsForMonitors([]string{"m1"}, 2)
			if err != nil {
				t.Fatal(err)
			}
			if d, ok := findDay(got["m1"], dayStr(1)); !ok || d.Total != 2 || d.UptimePercent != 50 {
				t.Fatalf("history after index change = %+v, found = %v", d, ok)
			}
		}
	})
}
