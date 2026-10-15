package uptime

import (
	"strings"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

func TestAdaptiveThreshold_UsesTheMonitorsOwnNormal(t *testing.T) {
	p := defaultBaselinePolicy()

	cases := []struct {
		name string
		b    db.LatencyBaseline
		want int64
		ok   bool
	}{
		{
			name: "typical service",
			b:    db.LatencyBaseline{P50: 254, P95: 420, Samples: 10000},
			want: 630,
			ok:   true,
		},
		{
			name: "fast service uses the same percentage",
			b:    db.LatencyBaseline{P50: 5, P95: 8, Samples: 10000},
			want: 12,
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
		{"monitor percentage overrides global percentage", &override, good, true, 2520},
		{"global percentage applies to learned baseline", nil, good, true, 630},
		{"thin baseline stays learning", nil, thin, true, 0},
		{"no baseline stays learning", nil, db.LatencyBaseline{}, false, 0},
	}

	for _, c := range cases {
		got := resolveLatencyThreshold(c.override, c.baseline, c.hasBaseline, p)
		if got != c.want {
			t.Errorf("%s: got %dms, want %dms", c.name, got, c.want)
		}
	}
}

func TestResolveLatencyThreshold_IgnoresEmptyOverride(t *testing.T) {
	p := defaultBaselinePolicy()
	zero := 0
	good := db.LatencyBaseline{P50: 254, P95: 420, Samples: 10000}

	if got := resolveLatencyThreshold(&zero, good, true, p); got != 630 {
		t.Errorf("a zero override should not suppress the baseline, got %dms", got)
	}
}

func TestLoadBaselinePolicy_ReadsSettings(t *testing.T) {
	m, store := newAlertTestManager(t)

	if got := m.loadBaselinePolicy(); got != defaultBaselinePolicy() {
		t.Errorf("with no settings: %+v, want the defaults", got)
	}

	_ = store.SetSetting("notification.latency.baseline_days", "14")
	_ = store.SetSetting("notification.latency.min_samples", "500")
	_ = store.SetSetting("notification.latency.increase_percent", "75")
	want := baselinePolicy{Window: 14 * 24 * time.Hour, MinSamples: 500, IncreasePercent: 75}
	if got := m.loadBaselinePolicy(); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	for _, invalid := range []string{"0", "-1", "9901", "1.5"} {
		_ = store.SetSetting("notification.latency.increase_percent", invalid)
		if got := m.loadBaselinePolicy(); got.IncreasePercent != 50 {
			t.Errorf("accepted %s", invalid)
		}
	}

}

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

	if got := m.monitors["m1"].GetLatencyThreshold(); got != 600 {
		t.Errorf("running monitor threshold = %dms, want 600ms", got)
	}

	if err := store.SetSetting("notification.latency.increase_percent", "25"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSetting("notification.latency.min_samples", "10"); err != nil {
		t.Fatal(err)
	}
	m.Sync()
	if got := m.GetMonitor("m1").GetLatencyThreshold(); got != 500 {
		t.Fatalf("global percentage change: got %dms, want 500ms", got)
	}
	override := 75
	for _, tc := range []struct {
		percent *int
		want    int64
	}{{&override, 700}, {nil, 500}} {
		if err := store.UpdateMonitor("m1", db.MonitorTypeHTTP, "API", "https://example.com", 60, nil, nil, tc.percent, nil); err != nil {
			t.Fatal(err)
		}
		m.Sync()
		if got := m.GetMonitor("m1").GetLatencyThreshold(); got != tc.want {
			t.Fatalf("changing override: got %dms, want %dms", got, tc.want)
		}
	}
}

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

func TestDegradedMessage_NamesTheBaseline(t *testing.T) {
	mon := NewMonitor("m1", db.MonitorTypeHTTP, "g1", "API", "https://api.example.com",
		time.Minute, nil, time.Now(), nil)

	mon.SetLatencyThreshold(1000)
	if got, want := mon.DegradedMessage(), "High latency detected (>1000ms)"; got != want {
		t.Errorf("without a baseline: %q, want %q", got, want)
	}

	mon.SetLatencyThreshold(630)
	mon.SetBaselineP95(254)
	if got, want := mon.DegradedMessage(), "High latency detected (>630ms, baseline p95 254ms)"; got != want {
		t.Errorf("with a baseline: %q, want %q", got, want)
	}
}

func TestAdaptiveThreshold_SteadyService(t *testing.T) {
	p := defaultBaselinePolicy()

	got, ok := p.adaptiveThreshold(db.LatencyBaseline{P50: 200, P95: 200, Samples: 10000})
	if !ok {
		t.Fatal("a steady service should still get a threshold")
	}
	if got != 300 {
		t.Errorf("threshold = %dms, want 300ms", got)
	}
}

func TestAdaptiveThreshold_SampleBoundary(t *testing.T) {
	p := defaultBaselinePolicy()

	if _, ok := p.adaptiveThreshold(db.LatencyBaseline{P95: 420, Samples: p.MinSamples - 1}); ok {
		t.Error("one sample short of the minimum was accepted")
	}
	if _, ok := p.adaptiveThreshold(db.LatencyBaseline{P95: 420, Samples: p.MinSamples}); !ok {
		t.Error("exactly at the minimum was rejected")
	}
}

func TestAdaptiveThreshold_VeryFastTarget(t *testing.T) {
	p := defaultBaselinePolicy()

	got, ok := p.adaptiveThreshold(db.LatencyBaseline{P50: 1, P95: 1, Samples: 10000})
	if !ok {
		t.Fatal("a fast target should still get a threshold")
	}
	if got != 2 {
		t.Errorf("threshold = %dms, want 2ms", got)
	}
}

func TestResolveLatencyThreshold_IgnoresANegativeOverride(t *testing.T) {
	p := defaultBaselinePolicy()
	negative := -500
	good := db.LatencyBaseline{P50: 254, P95: 420, Samples: 10000}

	if got := resolveLatencyThreshold(&negative, good, true, p); got != 630 {
		t.Errorf("a negative override took effect: %dms", got)
	}
}

func TestComputeLatencyBaseline_IdenticalSamples(t *testing.T) {
	_, store := newAlertTestManager(t)
	now := time.Now().UTC()

	var checks []db.CheckResult
	for i := 0; i < 50; i++ {
		checks = append(checks, db.CheckResult{
			MonitorID: "m1", Status: "up", Latency: 42,
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
		})
	}
	if err := store.BatchInsertChecks(checks); err != nil {
		t.Fatalf("BatchInsertChecks: %v", err)
	}

	if _, err := store.ComputeLatencyBaseline("m1", 7*24*time.Hour, 10, now); err != nil {
		t.Fatalf("ComputeLatencyBaseline: %v", err)
	}
	b, _, _ := store.GetLatencyBaseline("m1")
	if b.P50 != 42 || b.P95 != 42 {
		t.Errorf("p50/p95 = %d/%d, want 42/42", b.P50, b.P95)
	}
}

func TestComputeLatencyBaseline_AllChecksFailed(t *testing.T) {
	_, store := newAlertTestManager(t)
	now := time.Now().UTC()

	var checks []db.CheckResult
	for i := 0; i < 50; i++ {
		checks = append(checks, db.CheckResult{
			MonitorID: "m1", Status: "down", Latency: 5000,
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
		})
	}
	if err := store.BatchInsertChecks(checks); err != nil {
		t.Fatalf("BatchInsertChecks: %v", err)
	}

	written, err := store.ComputeLatencyBaseline("m1", 7*24*time.Hour, 1, now)
	if err != nil {
		t.Fatalf("ComputeLatencyBaseline: %v", err)
	}
	if written {
		t.Error("a baseline was derived from failed checks only")
	}
}

func TestRefreshBaselines_MonitorWithNoHistory(t *testing.T) {
	m, store := newAlertTestManager(t)

	m.refreshBaselines(time.Now().UTC())

	if _, ok, _ := store.GetLatencyBaseline("m1"); ok {
		t.Error("a baseline was stored for a monitor with no checks")
	}
	if got := m.monitors["m1"].GetLatencyThreshold(); got != 0 {
		t.Errorf("threshold = %dms, want learning (0)", got)
	}
}

func TestSync_AppliesThePersistedBaseline(t *testing.T) {
	_, store := newAlertTestManager(t)
	now := time.Now().UTC()

	var checks []db.CheckResult
	for i := 0; i < 300; i++ {
		latency := int64(250)
		if i%10 == 0 {
			latency = 400
		}
		checks = append(checks, db.CheckResult{
			MonitorID: "m1", Status: "up", Latency: latency,
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
		})
	}
	if err := store.BatchInsertChecks(checks); err != nil {
		t.Fatalf("BatchInsertChecks: %v", err)
	}
	if _, err := store.ComputeLatencyBaseline("m1", 7*24*time.Hour, 200, now); err != nil {
		t.Fatalf("ComputeLatencyBaseline: %v", err)
	}

	restarted := NewManager(store)
	t.Cleanup(restarted.Reset)
	restarted.Sync()

	mon := restarted.GetMonitor("m1")
	if mon == nil {
		t.Fatal("Sync did not schedule the monitor")
	}
	if got := mon.GetLatencyThreshold(); got != 600 {
		t.Errorf("threshold after Sync = %dms, want 600ms from the stored baseline", got)
	}
	if !strings.Contains(mon.DegradedMessage(), "baseline p95 400ms") {
		t.Errorf("the message lost the baseline after Sync: %q", mon.DegradedMessage())
	}
}

func TestSync_KeepsThePerMonitorOverride(t *testing.T) {
	_, store := newAlertTestManager(t)
	now := time.Now().UTC()

	override := 500
	if err := store.UpdateMonitor("m1", db.MonitorTypeHTTP, "Jellyfin", "https://jellyfin.example.com",
		60, nil, nil, &override, nil); err != nil {
		t.Fatalf("UpdateMonitor: %v", err)
	}

	var checks []db.CheckResult
	for i := 0; i < 300; i++ {
		checks = append(checks, db.CheckResult{
			MonitorID: "m1", Status: "up", Latency: 250,
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
		})
	}
	if err := store.BatchInsertChecks(checks); err != nil {
		t.Fatalf("BatchInsertChecks: %v", err)
	}
	if _, err := store.ComputeLatencyBaseline("m1", 7*24*time.Hour, 200, now); err != nil {
		t.Fatalf("ComputeLatencyBaseline: %v", err)
	}

	restarted := NewManager(store)
	t.Cleanup(restarted.Reset)
	restarted.Sync()

	if got := restarted.GetMonitor("m1").GetLatencyThreshold(); got != 1500 {
		t.Errorf("threshold = %dms, want the 250ms + 500%% = 1500ms", got)
	}
}

func TestDynamicThreshold_PercentageAndLearning(t *testing.T) {
	p := defaultBaselinePolicy()
	b := db.LatencyBaseline{P95: 1000, Samples: 200}
	percent := 25
	threshold := resolveLatencyThreshold(&percent, b, true, p)
	if threshold != 1250 {
		t.Fatalf("got %d, want 1250", threshold)
	}
	mon := NewMonitor("m", db.MonitorTypeHTTP, "g", "API", "https://example.com", time.Minute, nil, time.Now(), nil)
	mon.SetLatencyThreshold(threshold)
	if mon.IsHighLatency(1250) || !mon.IsHighLatency(1251) {
		t.Fatal("incorrect threshold boundary")
	}
	mon.SetLatencyThreshold(resolveLatencyThreshold(&percent, b, false, p))
	if mon.IsHighLatency(100000) {
		t.Fatal("learning monitor classified as slow")
	}
}

func TestDynamicLatency_ConfirmationAlertsRecoveryAndLearning(t *testing.T) {
	m, store := newAlertTestManager(t)
	mon := m.GetMonitor("m1")
	mon.ApplyConfig(MonitorConfig{ConfirmationThreshold: 2, RecoveryConfirmationChecks: 2})
	mon.SetLatencyThreshold(1250)
	mon.SetBaselineP95(1000)
	m.wg.Add(1)
	go m.resultProcessor()
	defer m.Stop()
	send := func(up bool, latency int64) {
		t.Helper()
		stamp := time.Now()
		m.resultQueue <- CheckResult{MonitorID: "m1", Status: up, Latency: latency, Timestamp: stamp}
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			history := mon.GetHistory()
			if len(history) > 0 && history[len(history)-1].Timestamp.Equal(stamp) {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("result was not processed")
	}
	assertOutages := func(want int, kind string) []db.OpenOutage {
		t.Helper()
		outages, err := store.GetOpenOutages()
		if err != nil || len(outages) != want {
			t.Fatalf("outages=%v err=%v, want %d", outages, err, want)
		}
		if want > 0 && outages[0].Type != kind {
			t.Fatalf("type=%s, want %s", outages[0].Type, kind)
		}
		return outages
	}
	send(true, 1250)
	assertOutages(0, "")
	send(true, 1251)
	assertOutages(0, "")
	send(true, 1400)
	outages := assertOutages(1, "degraded")
	m.evaluateAlerts(outages[0].StartTime.Add(2 * time.Minute))
	if notifiedAt(t, store) != nil {
		t.Fatal("alert bypassed sustained window")
	}
	m.evaluateAlerts(outages[0].StartTime.Add(3 * time.Minute))
	if notifiedAt(t, store) == nil {
		t.Fatal("confirmed degradation was not announced")
	}
	send(true, 1000)
	assertOutages(0, "")
	mon.SetLatencyThreshold(0)
	send(true, 100000)
	send(true, 100000)
	assertOutages(0, "")
	send(false, 0)
	assertOutages(0, "")
	send(false, 0)
	assertOutages(1, "down")
}

func TestDynamicThreshold_IntegerRounding(t *testing.T) {
	p := defaultBaselinePolicy()
	p.IncreasePercent = 10
	for _, tc := range []struct{ normal, want int64 }{{100, 110}, {101, 112}, {1, 2}} {
		got, ok := p.adaptiveThreshold(db.LatencyBaseline{P95: tc.normal, Samples: 200})
		if !ok || got != tc.want {
			t.Errorf("normal=%d: got %d, want %d", tc.normal, got, tc.want)
		}
	}
}
