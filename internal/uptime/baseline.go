package uptime

import (
	"log"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

// How often baselines are recomputed. What counts as normal for a service moves over days,
// not minutes, so hourly is generous.
const baselineInterval = time.Hour

// baselinePolicy turns a monitor's own history into the line above which it is considered
// slow. A single global number cannot serve a health check that answers in 254ms and a
// homepage that answers in 427ms: the first is broken at 600ms, the second is fine.
type baselinePolicy struct {
	// Enabled turns adaptive thresholds off entirely, falling back to the fixed number.
	Enabled bool
	// Window is how far back the percentiles look.
	Window time.Duration
	// MinSamples is how much history a monitor needs before its baseline is trusted.
	MinSamples int
	// Factor multiplies p95. p95 already absorbs a service's normal spikes, so exceeding a
	// multiple of it is genuinely unusual rather than merely the slow end of normal.
	Factor float64
	// FloorMs is added to p95 as an alternative bound, and the larger of the two wins. It
	// stops very fast targets from alerting on noise: a service whose p95 is 8ms should not
	// be called degraded at 12ms.
	FloorMs int64
}

func defaultBaselinePolicy() baselinePolicy {
	return baselinePolicy{
		Enabled:    true,
		Window:     7 * 24 * time.Hour,
		MinSamples: 200,
		Factor:     1.5,
		FloorMs:    100,
	}
}

// adaptiveThreshold is the latency above which this monitor counts as degraded, given what
// it normally does. Returns false when the baseline is too thin to trust.
func (p baselinePolicy) adaptiveThreshold(b db.LatencyBaseline) (int64, bool) {
	if !p.Enabled || b.Samples < p.MinSamples || b.P95 <= 0 {
		return 0, false
	}

	byFactor := int64(float64(b.P95) * p.Factor)
	byFloor := b.P95 + p.FloorMs
	if byFactor > byFloor {
		return byFactor, true
	}
	return byFloor, true
}

// resolveLatencyThreshold picks the number a monitor is judged against, most specific
// first: an explicit per-monitor value is a deliberate statement about that service — an
// SLA, usually — and outranks anything learned. Otherwise the monitor's own baseline, and
// failing that the global default, which is all a monitor with no history has.
//
// Repointing a monitor at a different target does not reset its baseline. It cannot: the
// old target's checks stay in the table by design — this codebase keeps a monitor's
// history across edits rather than discarding it — so any recomputation would derive the
// same numbers again. The baseline simply re-learns as the window rolls past the change.
// Anyone who needs the new target judged correctly today can set the per-monitor
// threshold, which outranks the baseline.
func resolveLatencyThreshold(override *int, baseline db.LatencyBaseline, hasBaseline bool, p baselinePolicy, globalDefault int64) int64 {
	if override != nil && *override > 0 {
		return int64(*override)
	}
	if hasBaseline {
		if v, ok := p.adaptiveThreshold(baseline); ok {
			return v
		}
	}
	return globalDefault
}

func (m *Manager) baselineWorker() {
	m.wg.Add(1)
	defer m.wg.Done()

	// Compute once at startup so a fresh install stops using the global default as soon as
	// it has enough history, rather than an hour later.
	m.refreshBaselines(time.Now())

	ticker := time.NewTicker(baselineInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.refreshBaselines(time.Now())
		}
	}
}

// refreshBaselines recomputes every running monitor's percentiles. Two queries per monitor
// per hour, so this stays cheap even with a few hundred monitors.
func (m *Manager) refreshBaselines(now time.Time) {
	m.mu.RLock()
	policy := m.baselinePolicy
	m.mu.RUnlock()

	if !policy.Enabled {
		return
	}

	for id := range m.GetAll() {
		// A monitor without enough history is skipped rather than given a shaky baseline;
		// it keeps using the global default until it has learned enough.
		if _, err := m.store.ComputeLatencyBaseline(id, policy.Window, policy.MinSamples, now); err != nil {
			log.Printf("Baseline: failed to compute for %s: %v", id, err)
		}
	}

	// Push the new numbers onto the running monitors in one pass, so the result processor
	// keeps reading a plain int64 on the hot path.
	m.applyLatencyThresholds()
}

// applyLatencyThresholds resolves and sets the effective threshold for every running
// monitor. Called after a baseline refresh and on every Sync, since either the override,
// the global default or the baseline may have moved.
func (m *Manager) applyLatencyThresholds() {
	baselines, err := m.store.GetLatencyBaselines()
	if err != nil {
		log.Printf("Baseline: failed to load baselines: %v", err)
		return
	}

	monitors, err := m.store.GetMonitors()
	if err != nil {
		log.Printf("Baseline: failed to load monitors: %v", err)
		return
	}

	m.mu.RLock()
	policy := m.baselinePolicy
	globalDefault := m.latencyThreshold
	m.mu.RUnlock()

	for _, dbM := range monitors {
		mon := m.GetMonitor(dbM.ID)
		if mon == nil {
			continue
		}
		b, has := baselines[dbM.ID]
		mon.SetLatencyThreshold(resolveLatencyThreshold(dbM.LatencyThreshold, b, has, policy, globalDefault))
		if has && policy.Enabled {
			mon.SetBaselineP50(b.P50)
		} else {
			mon.SetBaselineP50(0)
		}
	}
}

// loadBaselinePolicy reads the adaptive-threshold settings.
func (m *Manager) loadBaselinePolicy() baselinePolicy {
	p := defaultBaselinePolicy()

	if val, err := m.store.GetSetting("notification.latency.adaptive_enabled"); err == nil && val != "" {
		p.Enabled = val != "false"
	}
	if v, ok := m.settingInt("notification.latency.baseline_days"); ok && v >= 1 {
		p.Window = time.Duration(v) * 24 * time.Hour
	}
	if v, ok := m.settingInt("notification.latency.min_samples"); ok && v >= 1 {
		p.MinSamples = v
	}
	if v, ok := m.settingInt("notification.latency.factor_percent"); ok && v >= 100 {
		p.Factor = float64(v) / 100
	}
	if v, ok := m.settingInt("notification.latency.floor_ms"); ok {
		p.FloorMs = int64(v)
	}

	return p
}
