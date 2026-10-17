package config

import "testing"

func TestIncrementalUptimeOptIn(t *testing.T) {
	for _, value := range []string{"", "false", "true", "TRUE", "invalid"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("INCREMENTAL_UPTIME_ENABLED", value)
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			want := value == "true" || value == "TRUE"
			if cfg.IncrementalUptime != want {
				t.Fatalf("incremental=%v want=%v", cfg.IncrementalUptime, want)
			}
		})
	}
}
