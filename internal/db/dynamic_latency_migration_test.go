package db

import (
	"strings"
	"testing"
)

func TestDynamicLatencyMigration_PreservesUnits(t *testing.T) {
	for _, tc := range []struct{ factor, want string }{{"", "50"}, {"150", "50"}, {"125", "25"}, {"100", "50"}} {
		t.Run("factor_"+tc.factor, func(t *testing.T) {
			RunTestWithBothDBs(t, "dynamic_latency", func(t *testing.T, s *Store) {
				if err := s.CreateGroup(Group{ID: "g-default", Name: "Default"}); err != nil {
					t.Fatal(err)
				}
				migrationFS, path := sqliteMigrationFS, "migrations/sqlite/00035_dynamic_latency.sql"
				if s.IsPostgres() {
					migrationFS, path = postgresMigrationFS, "migrations/postgres/00035_dynamic_latency.sql"
				}
				script, err := migrationFS.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				parts := strings.Split(string(script), "-- +goose Down")
				if _, err := s.db.Exec(parts[1]); err != nil {
					t.Fatal(err)
				}
				if tc.factor != "" {
					if err := s.SetSetting("notification.latency.factor_percent", tc.factor); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := s.db.Exec(s.rebind("INSERT INTO monitors (id, group_id, name, url, latency_threshold) VALUES (?, ?, ?, ?, ?)"), "migration-latency", "g-default", "API", "https://example.com", 1000); err != nil {
					t.Fatal(err)
				}
				if _, err := s.db.Exec(parts[0]); err != nil {
					t.Fatal(err)
				}
				got, err := s.GetSetting("notification.latency.increase_percent")
				if err != nil || got != tc.want {
					t.Fatalf("percentage=%q err=%v, want %q", got, err, tc.want)
				}
				var legacy int
				var inherits bool
				if err := s.db.QueryRow("SELECT latency_threshold, latency_increase_percent IS NULL FROM monitors WHERE id = 'migration-latency'").Scan(&legacy, &inherits); err != nil {
					t.Fatal(err)
				}
				if legacy != 1000 || !inherits {
					t.Fatalf("legacy=%d inherits=%v", legacy, inherits)
				}
			})
		})
	}
}
