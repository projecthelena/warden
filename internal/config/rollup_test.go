package config

import "testing"

func TestRollupDiagnosticsOptIn(t *testing.T) {
	for _, value := range []string{"", "false", "true", "TRUE", "invalid"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("ROLLUP_DIAGNOSTICS_ENABLED", value)
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			want := value == "true" || value == "TRUE"
			if cfg.RollupDiagnostics != want {
				t.Fatalf("diagnostics = %v, want %v", cfg.RollupDiagnostics, want)
			}
		})
	}
}
