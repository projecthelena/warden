package uptime

import (
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

func TestAdaptiveThreshold_UsesTheMonitorsOwnNormal(t *testing.T) {
	p := defaultBaselinePolicy() // factor 1.5, floor 100ms, min 200 samples

	cases := []struct {
		name string
		b    db.LatencyBaseline
		want int64
		ok   bool
	}{
		{
			// The real shape of homedepot-nucleus-prod-3: a flat 254ms baseline with ramps.
			// The factor governs, and 650ms — already 2.5x its normal — now counts as
			// degraded, where the fixed 1000ms threshold said nothing.
			name: "typical service",
			b:    db.LatencyBaseline{P50: 254, P95: 420, Samples: 10000},
			want: 630,
			ok:   true,
		},
		{
			// A very fast target must not be called degraded over a few milliseconds of
			// noise, so the absolute floor takes over from the multiplier.
			name: "fast service, floor governs",
			b:    db.LatencyBaseline{P50: 5, P95: 8, Samples: 10000},
			want: 108,
			ok:   true,
		},
		{
			name: "slow but consistent service",
			b:    db.LatencyBaseline{P50: 427, P95: 900, Samples: 10000},
			want: 1350,
			ok:   true,
		},
		{
			name: "not enough history to trust",
			b:    db.LatencyBaseline{P50: 254, P95: 420, Samples: 12},
			ok:   false,
		},
		{
			name: "no usable percentile",
			b:    db.LatencyBaseline{P50: 0, P95: 0, Samples: 10000},
			ok:   false,
		},
	}

	for _, c := range cases {
		got, ok := p.adaptiveThreshold(c.b)
		if ok != c.ok {
			t.Errorf("%s: usable = %v, want %v", c.name, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("%s: threshold = %dms, want %dms", c.name, got, c.want)
		}
	}
}

func TestAdaptiveThreshold_DisabledFallsBack(t *testing.T) {
	p := defaultBaselinePolicy()
	p.Enabled = false
	if _, ok := p.adaptiveThreshold(db.LatencyBaseline{P50: 254, P95: 420, Samples: 10000}); ok {
		t.Error("adaptive thresholds should be unusable when the feature is off")
	}
}

// An explicit per-monitor number is a deliberate statement about that service — usually an
// SLA — so nothing learned may quietly overrule it.
func TestResolveLatencyThreshold_Precedence(t *testing.T) {
	p := defaultBaselinePolicy()
	good := db.LatencyBaseline{P50: 254, P95: 420, Samples: 10000}
	thin := db.LatencyBaseline{P50: 254, P95: 420, Samples: 3}
	override := 500

	cases := []struct {
		name        string
		override    *int
		baseline    db.LatencyBaseline
		hasBaseline bool
		want        int64
	}{
		{"explicit override wins over the baseline", &override, good, true, 500},
		{"baseline wins over the global default", nil, good, true, 630},
		{"thin baseline falls back to the global default", nil, thin, true, 1000},
		{"no baseline at all falls back", nil, db.LatencyBaseline{}, false, 1000},
	}

	for _, c := range cases {
		got := resolveLatencyThreshold(c.override, c.baseline, c.hasBaseline, p, 1000)
		if got != c.want {
			t.Errorf("%s: got %dms, want %dms", c.name, got, c.want)
		}
	}
}

// A zero or negative override is not an override — it is an empty field.
func TestResolveLatencyThreshold_IgnoresEmptyOverride(t *testing.T) {
	p := defaultBaselinePolicy()
	zero := 0
	good := db.LatencyBaseline{P50: 254, P95: 420, Samples: 10000}

	if got := resolveLatencyThreshold(&zero, good, true, p, 1000); got != 630 {
		t.Errorf("a zero override should not suppress the baseline, got %dms", got)
	}
}

func TestLoadBaselinePolicy_ReadsSettings(t *testing.T) {
	m, store := newAlertTestManager(t)

	if got := m.loadBaselinePolicy(); got != defaultBaselinePolicy() {
		t.Errorf("with no settings: %+v, want the defaults", got)
	}

	_ = store.SetSetting("notification.latency.adaptive_enabled", "false")
	_ = store.SetSetting("notification.latency.baseline_days", "14")
	_ = store.SetSetting("notification.latency.min_samples", "500")
	_ = store.SetSetting("notification.latency.factor_percent", "200")
	_ = store.SetSetting("notification.latency.floor_ms", "50")

	want := baselinePolicy{
		Enabled:    false,
		Window:     14 * 24 * time.Hour,
		MinSamples: 500,
		Factor:     2,
		FloorMs:    50,
	}
	if got := m.loadBaselinePolicy(); got != want {
		t.Errorf("loadBaselinePolicy = %+v, want %+v", got, want)
	}

	// A factor below 1 would mark a service degraded at its own median, so it is rejected.
	_ = store.SetSetting("notification.latency.factor_percent", "50")
	if got := m.loadBaselinePolicy(); got.Factor != defaultBaselinePolicy().Factor {
		t.Errorf("a factor below 1x was accepted: %v", got.Factor)
	}
}

// The end-to-end path: a monitor with real history gets a threshold from its own numbers,
// and the running monitor is actually told about it.
func TestRefreshBaselines_AppliesToRunningMonitors(t *testing.T) {
	m, store := newAlertTestManager(t)
	m.baselinePolicy.MinSamples = 10

	now := time.Now().UTC()
	var checks []db.CheckResult
	for i := 0; i < 100; i++ {
		latency := int64(250)
		if i%10 == 0 {
			latency = 400 // the odd slow one, which is what p95 is for
		}
		checks = append(checks, db.CheckResult{
			MonitorID: "m1", Status: "up", Latency: latency,
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
		})
	}
	if err := store.BatchInsertChecks(checks); err != nil {
		t.Fatalf("BatchInsertChecks: %v", err)
	}

	m.refreshBaselines(now)

	b, ok, err := store.GetLatencyBaseline("m1")
	if err != nil || !ok {
		t.Fatalf("no baseline stored: ok=%v err=%v", ok, err)
	}
	if b.P50 != 250 {
		t.Errorf("p50 = %d, want 250", b.P50)
	}
	if b.P95 != 400 {
		t.Errorf("p95 = %d, want 400", b.P95)
	}

	// 400 * 1.5 = 600, versus 400 + 100 = 500. The multiplier governs.
	if got := m.monitors["m1"].GetLatencyThreshold(); got != 600 {
		t.Errorf("running monitor threshold = %dms, want 600ms", got)
	}
}

// Failed checks measure how long it took to fail, which says nothing about how fast the
// service is when it works. Letting them into the baseline would inflate "normal" exactly
// when the monitor is in trouble.
func TestComputeLatencyBaseline_IgnoresFailedChecks(t *testing.T) {
	m, store := newAlertTestManager(t)
	now := time.Now().UTC()

	var checks []db.CheckResult
	for i := 0; i < 50; i++ {
		checks = append(checks, db.CheckResult{
			MonitorID: "m1", Status: "up", Latency: 100,
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
		})
	}
	for i := 0; i < 50; i++ {
		checks = append(checks, db.CheckResult{
			MonitorID: "m1", Status: "down", Latency: 5000,
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
		})
	}
	if err := store.BatchInsertChecks(checks); err != nil {
		t.Fatalf("BatchInsertChecks: %v", err)
	}

	written, err := store.ComputeLatencyBaseline("m1", 7*24*time.Hour, 10, now)
	if err != nil || !written {
		t.Fatalf("ComputeLatencyBaseline: written=%v err=%v", written, err)
	}

	b, _, _ := store.GetLatencyBaseline("m1")
	if b.P95 != 100 {
		t.Errorf("p95 = %d, want 100 — timeouts leaked into the baseline", b.P95)
	}
	if b.Samples != 50 {
		t.Errorf("samples = %d, want 50", b.Samples)
	}
	_ = m
}

// A thin baseline is worse than none, because everything downstream trusts it.
func TestComputeLatencyBaseline_SkipsThinHistory(t *testing.T) {
	_, store := newAlertTestManager(t)
	now := time.Now().UTC()

	if err := store.BatchInsertChecks([]db.CheckResult{
		{MonitorID: "m1", Status: "up", Latency: 100, Timestamp: now},
	}); err != nil {
		t.Fatalf("BatchInsertChecks: %v", err)
	}

	written, err := store.ComputeLatencyBaseline("m1", 7*24*time.Hour, 200, now)
	if err != nil {
		t.Fatalf("ComputeLatencyBaseline: %v", err)
	}
	if written {
		t.Error("a baseline was written from a single sample")
	}
	if _, ok, _ := store.GetLatencyBaseline("m1"); ok {
		t.Error("a thin baseline was stored anyway")
	}
}

// An adaptive threshold is a number with no story unless the message says what normal is:
// the reader cannot otherwise tell whether ">630ms" is twice this service's usual or a
// tenth of it.
func TestDegradedMessage_NamesTheBaseline(t *testing.T) {
	mon := NewMonitor("m1", db.MonitorTypeHTTP, "g1", "API", "https://api.example.com",
		time.Minute, nil, time.Now(), nil)

	mon.SetLatencyThreshold(1000)
	if got, want := mon.DegradedMessage(), "High latency detected (>1000ms)"; got != want {
		t.Errorf("without a baseline: %q, want %q", got, want)
	}

	mon.SetLatencyThreshold(630)
	mon.SetBaselineP50(254)
	if got, want := mon.DegradedMessage(), "High latency detected (>630ms, normally ~254ms)"; got != want {
		t.Errorf("with a baseline: %q, want %q", got, want)
	}
}
