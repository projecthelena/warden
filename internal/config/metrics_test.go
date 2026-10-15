package config

import "testing"

func TestMetricsEnabled(t *testing.T) {
	for _, value := range []string{"", "true", "false", "TRUE", "0", "invalid"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("METRICS_ENABLED", value)
			cfg, err := Load()
			if value == "invalid" {
				if err == nil {
					t.Fatal("invalid switch accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := value != "false" && value != "0"
			if cfg.MetricsEnabled != want {
				t.Fatalf("enabled = %v, want %v", cfg.MetricsEnabled, want)
			}
		})
	}
}
