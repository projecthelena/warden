package db

import (
	"strings"
	"testing"
	"time"
)

func TestNotificationCooldownPersistsBetweenCallers(t *testing.T) {
	RunTestWithBothDBs(t, "notification cooldown", func(t *testing.T, s *Store) {
		seedOutageFixture(t, s)
		now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
		for _, tc := range []struct {
			age  time.Duration
			want bool
		}{
			{0, true}, {0, false}, {23 * time.Hour, false}, {-time.Hour, false}, {24 * time.Hour, true}, {25 * time.Hour, false},
		} {
			got, err := s.ClaimNotificationCooldown("m1", "flapping", now.Add(tc.age), 24*time.Hour)
			if err != nil || got != tc.want {
				t.Fatalf("after %s: claimed=%v, err=%v; want %v", tc.age, got, err, tc.want)
			}
		}
	})
}

func TestNoiseMigrationPreservesCustomReminderSchedules(t *testing.T) {
	RunTestWithBothDBs(t, "reminder migration", func(t *testing.T, s *Store) {
		migrationFS, path := sqliteMigrationFS, "migrations/sqlite/00034_notification_noise.sql"
		if s.IsPostgres() {
			migrationFS, path = postgresMigrationFS, "migrations/postgres/00034_notification_noise.sql"
		}
		data, err := migrationFS.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sql := string(data)
		start := strings.Index(sql, "UPDATE settings")
		end := strings.Index(sql[start:], ";") + start + 1
		for _, tc := range []struct{ first, repeat, wantFirst, wantRepeat string }{
			{"30", "60", "0", "0"}, {"30", "120", "30", "120"}, {"15", "60", "15", "60"}, {"0", "60", "0", "60"},
		} {
			if err := s.SetSetting("notification.alert.reminder_minutes", tc.first); err != nil {
				t.Fatal(err)
			}
			if err := s.SetSetting("notification.alert.repeat_reminder_minutes", tc.repeat); err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.Exec(sql[start:end]); err != nil {
				t.Fatal(err)
			}
			first, _ := s.GetSetting("notification.alert.reminder_minutes")
			repeat, _ := s.GetSetting("notification.alert.repeat_reminder_minutes")
			if first != tc.wantFirst || repeat != tc.wantRepeat {
				t.Fatalf("%s/%s migrated to %s/%s; want %s/%s", tc.first, tc.repeat, first, repeat, tc.wantFirst, tc.wantRepeat)
			}
		}
	})
}

func TestNoiseUpgradeDisablesLegacyFlappingNotices(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	if value, err := s.GetSetting("notification.event.flapping.enabled"); err != nil || value != "false" {
		t.Fatalf("fresh-install default: %q %v", value, err)
	}
	if err := s.SetSetting("notification.event.flapping.enabled", "true"); err != nil {
		t.Fatal(err)
	}
	data, err := sqliteMigrationFS.ReadFile("migrations/sqlite/00034_notification_noise.sql")
	if err != nil {
		t.Fatal(err)
	}
	statement := string(data)
	start := strings.Index(statement, "INSERT INTO settings")
	end := start + strings.Index(statement[start:], ";") + 1
	if _, err := s.db.Exec(statement[start:end]); err != nil {
		t.Fatal(err)
	}
	if value, err := s.GetSetting("notification.event.flapping.enabled"); err != nil || value != "false" {
		t.Fatalf("upgrade left notices enabled: %q %v", value, err)
	}
}
