package config

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestLoadSQLiteRollupBatchSize(t *testing.T) {
	for _, tc := range []struct {
		value   string
		want    int
		invalid bool
	}{
		{"", 50, false}, {"1", 1, false}, {"10", 10, false}, {"50", 50, false},
		{"0", 50, true}, {"-1", 50, true}, {"51", 50, true}, {"abc", 50, true},
		{"10.5", 50, true}, {" 10", 50, true}, {"999999999999999999999", 50, true},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("DB_TYPE", "sqlite")
			t.Setenv("DB_URL", "")
			t.Setenv("SQLITE_ROLLUP_BATCH_SIZE", tc.value)
			var logs bytes.Buffer
			previous := log.Writer()
			log.SetOutput(&logs)
			t.Cleanup(func() { log.SetOutput(previous) })
			cfg, err := Load()
			warned := strings.Contains(logs.String(), "invalid SQLITE_ROLLUP_BATCH_SIZE") && strings.Contains(logs.String(), "using default 50")
			if warned != tc.invalid {
				t.Fatalf("unexpected warning: %q", logs.String())
			}

			if err != nil {
				t.Fatal(err)
			}
			if cfg.SQLiteRollupBatchSize != tc.want {
				t.Fatalf("batch size = %d, want %d", cfg.SQLiteRollupBatchSize, tc.want)
			}
		})
	}
}

func TestLoadPostgresIgnoresSQLiteRollupBatchSize(t *testing.T) {
	for _, dialect := range []string{"postgres", "postgresql", ""} {
		t.Run(dialect, func(t *testing.T) {
			t.Setenv("DB_TYPE", dialect)
			t.Setenv("DB_URL", "postgres://localhost/warden")
			t.Setenv("SQLITE_ROLLUP_BATCH_SIZE", "invalid")
			var logs bytes.Buffer
			previous := log.Writer()
			log.SetOutput(&logs)
			t.Cleanup(func() { log.SetOutput(previous) })
			if _, err := Load(); err != nil {
				t.Fatal(err)
			}
			if logs.Len() != 0 {
				t.Fatalf("PostgreSQL must ignore SQLite setting: %s", logs.String())
			}
		})
	}
}
