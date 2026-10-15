package uptime

import (
	"log"
	"math"
	"time"

	"github.com/projecthelena/warden/internal/operationmetrics"

	"github.com/projecthelena/warden/internal/db"
)

// How often baselines are recomputed. What counts as normal for a service moves over days,
// not minutes, so hourly is generous.
const baselineInterval = time.Hour

// baselinePolicy turns a monitor's own history into the line above which it is considered
// slow. A single global number cannot serve a health check that answers in 254ms and a
// homepage that answers in 427ms: the first is broken at 600ms, the second is fine.
type baselinePolicy struct {
	Window          time.Duration
	MinSamples      int
	IncreasePercent int
	// LearnAfter excludes history from an old network or region after an operator asks
	// Warden to relearn. Zero means the regular rolling window applies.
	LearnAfter time.Time
}

func defaultBaselinePolicy() baselinePolicy {
	return baselinePolicy{
		Window:          7 * 24 * time.Hour,
		MinSamples:      200,
		IncreasePercent: 50,
	}
}

// adaptiveThreshold is the latency above which this monitor counts as degraded, given what
// it normally does. Returns false when the baseline is too thin to trust.
func (p baselinePolicy) adaptiveThreshold(b db.LatencyBaseline) (int64, bool) {
	if b.Samples < p.MinSamples || b.P95 <= 0 {
		return 0, false
	}
	factor := int64(100 + p.IncreasePercent)
	if b.P95 > (math.MaxInt64-99)/factor {
		return math.MaxInt64, true
	}
	return (b.P95*factor + 99) / 100, true
}

// Zero means learning: successful checks cannot be classified as slow yet.
func resolveLatencyThreshold(override *int, baseline db.LatencyBaseline, hasBaseline bool, p baselinePolicy) int64 {
	if override != nil && *override >= 1 && *override <= 9900 {
		p.IncreasePercent = *override
	}
	if hasBaseline {
		if v, ok := p.adaptiveThreshold(baseline); ok {
			return v
		}
	}
	return 0
}

func (m *Manager) baselineWorker() {
	m.wg.Add(1)
	defer m.wg.Done()

	// Compute once at startup so a fresh install stops learning as soon as
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

// refreshBaselines recomputes every running monitor's percentiles once per hour.
func (m *Manager) refreshBaselines(now time.Time) {
	m.mu.RLock()
	policy := m.baselinePolicy
	m.mu.RUnlock()

	finish := operationmetrics.Start(operationmetrics.BaselineCycle)
	var cycleErr error
	defer func() { finish(cycleErr) }()

	for id := range m.GetAll() {
		// A monitor without enough history is skipped rather than given a shaky baseline;
		// latency warnings wait until it has learned enough.
		since := now.Add(-policy.Window)
		if policy.LearnAfter.After(since) {
			since = policy.LearnAfter
		}
		updated, err := m.store.ComputeLatencyBaselineSince(id, since, policy.MinSamples, now)
		outcome := "skipped"
		if updated {
			outcome = "updated"
		}
		if err != nil {
			outcome = "error"
			cycleErr = err
			log.Printf("Baseline: failed to compute for %s: %v", id, err)
		}
		operationmetrics.BaselineMonitors.WithLabelValues(outcome).Inc()
	}

	// Push the new numbers onto the running monitors in one pass, so the result processor
	// keeps reading a plain int64 on the hot path.
	if err := m.applyLatencyThresholds(); err != nil {
		cycleErr = err
	}
}

// applyLatencyThresholds resolves and sets the effective threshold for every running
// monitor. Called after a baseline refresh and on every Sync, since either the override,
// the global default or the baseline may have moved.
func (m *Manager) applyLatencyThresholds() error {
	baselines, err := m.store.GetLatencyBaselines()
	if err != nil {
		log.Printf("Baseline: failed to load baselines: %v", err)
		return err
	}

	monitors, err := m.store.GetMonitors()
	if err != nil {
		log.Printf("Baseline: failed to load monitors: %v", err)
		return err
	}

	m.mu.RLock()
	policy := m.baselinePolicy
	m.mu.RUnlock()

	for _, dbM := range monitors {
		mon := m.GetMonitor(dbM.ID)
		if mon == nil {
			continue
		}
		b, has := baselines[dbM.ID]
		mon.SetLatencyThreshold(resolveLatencyThreshold(dbM.LatencyIncreasePercent, b, has, policy))
		if has && mon.GetLatencyThreshold() > 0 {
			mon.SetBaselineP95(b.P95)
		} else {
			mon.SetBaselineP95(0)
		}
	}
	return nil
}

// loadBaselinePolicy reads the adaptive-threshold settings.
func (m *Manager) loadBaselinePolicy() baselinePolicy {
	p := defaultBaselinePolicy()

	if v, ok := m.settingInt("notification.latency.baseline_days"); ok && v >= 1 {
		p.Window = time.Duration(v) * 24 * time.Hour
	}
	if v, ok := m.settingInt("notification.latency.min_samples"); ok && v >= 1 {
		p.MinSamples = v
	}
	if v, ok := m.settingInt("notification.latency.increase_percent"); ok && v >= 1 && v <= 9900 {
		p.IncreasePercent = v
	}
	if val, err := m.store.GetSetting("notification.latency.learn_after"); err == nil && val != "" {
		if cutoff, err := time.Parse(time.RFC3339Nano, val); err == nil {
			p.LearnAfter = cutoff
		}
	}

	return p
}

// RelearnLatencyBaselines starts adaptive learning over with checks made from now on.
// Historical checks remain in the database for charts and uptime reports.
func (m *Manager) RelearnLatencyBaselines(now time.Time) error {
	if err := m.store.RelearnLatencyBaselines(now); err != nil {
		return err
	}

	m.mu.Lock()
	m.baselinePolicy.LearnAfter = now.UTC()
	monitors := make([]*Monitor, 0, len(m.monitors))
	for _, mon := range m.monitors {
		monitors = append(monitors, mon)
	}
	m.mu.Unlock()

	for _, mon := range monitors {
		mon.ForgetDegradedState()
	}
	_ = m.applyLatencyThresholds()
	return nil
}
