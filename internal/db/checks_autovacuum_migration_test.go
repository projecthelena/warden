package db

import (
	"strings"
	"testing"
)

func TestChecksAutovacuumMigration_RoundTrip(t *testing.T) {
	RunTestWithBothDBs(t, "checks autovacuum migration", func(t *testing.T, s *Store) {
		if !s.IsPostgres() {
			t.Skip("autovacuum settings only exist on PostgreSQL")
		}
		reloptions := func() string {
			var opts string
			if err := s.db.QueryRow("SELECT coalesce(array_to_string(reloptions, ','), '') FROM pg_class WHERE oid = 'monitor_checks'::regclass").Scan(&opts); err != nil {
				t.Fatal(err)
			}
			return opts
		}
		want := "autovacuum_vacuum_insert_threshold=10000,autovacuum_vacuum_insert_scale_factor=0"
		if got := reloptions(); got != want {
			t.Fatalf("reloptions after migrations = %q, want %q", got, want)
		}

		script, err := postgresMigrationFS.ReadFile("migrations/postgres/00038_monitor_checks_autovacuum.sql")
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.Split(string(script), "-- +goose Down")
		if _, err := s.db.Exec(parts[1]); err != nil {
			t.Fatal(err)
		}
		if got := reloptions(); got != "" {
			t.Fatalf("reloptions after down = %q, want none", got)
		}
		if _, err := s.db.Exec(parts[0]); err != nil {
			t.Fatal(err)
		}
		if got := reloptions(); got != want {
			t.Fatalf("reloptions after up = %q, want %q", got, want)
		}
	})
}
