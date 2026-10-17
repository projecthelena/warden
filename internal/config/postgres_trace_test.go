package config

import "testing"

func TestPostgresTraceOptIn(t *testing.T) {
	for _, value := range []string{"", "false", "true", "TRUE"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("POSTGRES_TRACE_ENABLED", value)
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.PostgresTrace != (value == "true" || value == "TRUE") {
				t.Fatalf("trace=%v for %q", cfg.PostgresTrace, value)
			}
		})
	}
}
