package uptime

import (
	"sync"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

type Status struct {
	Timestamp  time.Time `json:"timestamp"`
	IsUp       bool      `json:"isUp"`
	Latency    int64     `json:"latencyMs"` // milliseconds
	StatusCode int       `json:"statusCode"`
	Error      string    `json:"error,omitempty"`
	IsDegraded bool      `json:"isDegraded"`
}

type Monitor struct {
	id        string
	groupID   string
	name      string
	url       string
	interval  time.Duration
	createdAt time.Time
	history   []Status
	mu        sync.RWMutex
	stopCh    chan struct{}
	stopOnce  sync.Once
	jobQueue      chan<- Job
	requestConfig *db.RequestConfig

	// Notification fatigue state (protected by mu)
	confirmationThreshold int   // effective threshold (resolved from per-monitor or global)
	cooldownMinutes       int   // effective cooldown (resolved from per-monitor or global)
	latencyThreshold      int64 // effective latency threshold (resolved from per-monitor or global)

	consecutiveDownCount int  // consecutive failed checks
	consecutiveDegCount  int  // consecutive degraded checks
	confirmedDown        bool // threshold met for down
	confirmedDegraded    bool // threshold met for degraded

	lastNotifiedAt map[string]time.Time // per-event-type cooldown tracking
	isFlapping     bool                 // current flap state
	flapStabilizedAt time.Time          // when flapping last stopped (grace period)

	// Flap detection settings
	flapDetectionEnabled bool
	flapWindowChecks     int
	flapThresholdPercent int

	// Recovery confirmation
	recoveryConfirmationChecks int
	consecutiveUpCount         int
}

// NotificationEventFilter holds per-event-type notification toggle state.
type NotificationEventFilter struct {
	DownEnabled       bool
	UpEnabled         bool
	DegradedEnabled   bool
	FlappingEnabled   bool
	StabilizedEnabled bool
	SSLExpiringEnabled bool
}

// IsEnabled checks whether notifications for the given event type are enabled.
func (f *NotificationEventFilter) IsEnabled(eventType string) bool {
	switch eventType {
	case "down":
		return f.DownEnabled
	case "up":
		return f.UpEnabled
	case "degraded":
		return f.DegradedEnabled
	case "flapping":
		return f.FlappingEnabled
	case "stabilized":
		return f.StabilizedEnabled
	case "ssl_expiring":
		return f.SSLExpiringEnabled
	default:
		return true
	}
}

// MonitorConfig holds per-monitor notification fatigue settings.
type MonitorConfig struct {
	ConfirmationThreshold      int
	CooldownMinutes            int
	FlapDetectionEnabled       bool
	FlapWindowChecks           int
	FlapThresholdPercent       int
	RecoveryConfirmationChecks int
}

func NewMonitor(id, groupID, name, url string, interval time.Duration, jobQueue chan<- Job, createdAt time.Time, reqConfig *db.RequestConfig) *Monitor {
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	return &Monitor{
		id:                    id,
		groupID:               groupID,
		name:                  name,
		url:                   url,
		interval:              interval,
		createdAt:             createdAt,
		history:               make([]Status, 0, 50), // Keep last 50 in memory
		stopCh:                make(chan struct{}),
		jobQueue:              jobQueue,
		requestConfig:         reqConfig,
		confirmationThreshold: 3,  // default
		cooldownMinutes:       30, // default
		lastNotifiedAt:        make(map[string]time.Time),
		flapDetectionEnabled:       true,
		flapWindowChecks:           21,
		flapThresholdPercent:       25,
		recoveryConfirmationChecks: 1,
	}
}

// ApplyConfig sets notification fatigue configuration on a monitor.
func (m *Monitor) ApplyConfig(cfg MonitorConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.confirmationThreshold = cfg.ConfirmationThreshold
	m.cooldownMinutes = cfg.CooldownMinutes
	m.flapDetectionEnabled = cfg.FlapDetectionEnabled
	m.flapWindowChecks = cfg.FlapWindowChecks
	m.flapThresholdPercent = cfg.FlapThresholdPercent
	if cfg.RecoveryConfirmationChecks >= 1 {
		m.recoveryConfirmationChecks = cfg.RecoveryConfirmationChecks
	}
}

// alignDelay computes the duration until the next tick aligned to createdAt.
// Checks should fire at createdAt, createdAt+interval, createdAt+2*interval, ...
// At time now, the next aligned tick is interval - ((now - createdAt) mod interval).
func alignDelay(createdAt time.Time, interval time.Duration, now time.Time) time.Duration {
	elapsed := now.Sub(createdAt) % interval
	if elapsed < 0 {
		elapsed += interval // Handle clock skew
	}
	delay := interval - elapsed
	if delay == interval {
		delay = 0 // We're exactly on an aligned tick
	}
	return delay
}

func (m *Monitor) Start() {
	// Immediate check for instant feedback
	m.schedule()

	// Calculate delay to next aligned tick based on createdAt
	delay := alignDelay(m.createdAt, m.interval, time.Now())

	if delay > 0 {
		// Wait for alignment, then fire the aligned tick
		alignTimer := time.NewTimer(delay)
		defer alignTimer.Stop()

		select {
		case <-m.stopCh:
			return
		case <-alignTimer.C:
			m.schedule()
		}
	}

	// Start regular ticker from the now-aligned point
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.schedule()
		}
	}
}

func (m *Monitor) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
}

func (m *Monitor) schedule() {
	defer func() {
		if r := recover(); r != nil {
			// Ignore panic on closed channel
			_ = r
		}
	}()
	m.mu.RLock()
	cfg := m.requestConfig
	m.mu.RUnlock()
	select {
	case m.jobQueue <- Job{MonitorID: m.id, URL: m.url, RequestConfig: cfg}:
		// Scheduled
	default:
		// Queue full, skip this tick to avoid blocking scheduler
	}
}

// GetRequestConfig returns the monitor's request configuration.
func (m *Monitor) GetRequestConfig() *db.RequestConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.requestConfig
}

// RecordResult is called by the ResultProcessor to update in-memory history
func (m *Monitor) RecordResult(isUp bool, latency int64, ts time.Time, statusCode int, errStr string, isDegraded bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	status := Status{
		Timestamp:  ts,
		Latency:    latency,
		IsUp:       isUp,
		StatusCode: statusCode,
		Error:      errStr,
		IsDegraded: isDegraded,
	}

	// Keep last 50 checks
	if len(m.history) >= 50 {
		m.history = m.history[1:]
	}
	m.history = append(m.history, status)
}

func (m *Monitor) GetHistory() []Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Return copy
	dst := make([]Status, len(m.history))
	copy(dst, m.history)
	return dst
}

func (m *Monitor) GetName() string {
	return m.name
}

func (m *Monitor) GetTargetURL() string {
	return m.url
}

func (m *Monitor) GetGroupID() string {
	return m.groupID
}

func (m *Monitor) GetInterval() time.Duration {
	return m.interval
}

func (m *Monitor) GetLastStatus() (bool, int64, bool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.history) == 0 {
		return false, 0, false, false // No history yet
	}
	last := m.history[len(m.history)-1]
	return last.IsUp, last.Latency, true, last.IsDegraded
}

// --- Notification fatigue methods ---

// IncrementDown increments the consecutive down counter. Returns true if the
// confirmation threshold is now met (i.e., the monitor should transition to confirmed down).
func (m *Monitor) IncrementDown() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.consecutiveDownCount++
	if !m.confirmedDown && m.consecutiveDownCount >= m.confirmationThreshold {
		m.confirmedDown = true
		return true
	}
	return false
}

// IncrementDegraded increments the consecutive degraded counter. Returns true if the
// confirmation threshold is now met.
func (m *Monitor) IncrementDegraded() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.consecutiveDegCount++
	if !m.confirmedDegraded && m.consecutiveDegCount >= m.confirmationThreshold {
		m.confirmedDegraded = true
		return true
	}
	return false
}

// ResetDown resets the consecutive down counter and confirmed state. Returns true
// if the monitor was previously confirmed down (i.e., recovery should be notified).
// Also clears the "down" cooldown so a new incident will always notify.
func (m *Monitor) ResetDown() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	wasConfirmed := m.confirmedDown
	m.consecutiveDownCount = 0
	m.confirmedDown = false
	if wasConfirmed {
		delete(m.lastNotifiedAt, "down")
	}
	return wasConfirmed
}

// ResetDegraded resets the consecutive degraded counter and confirmed state. Returns
// true if the monitor was previously confirmed degraded.
// Also clears the "degraded" cooldown so a new degradation will always notify.
func (m *Monitor) ResetDegraded() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	wasConfirmed := m.confirmedDegraded
	m.consecutiveDegCount = 0
	m.confirmedDegraded = false
	if wasConfirmed {
		delete(m.lastNotifiedAt, "degraded")
	}
	return wasConfirmed
}

// IsConfirmedDown returns whether the monitor has met the down confirmation threshold.
func (m *Monitor) IsConfirmedDown() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.confirmedDown
}

// IsConfirmedDegraded returns whether the monitor has met the degraded confirmation threshold.
func (m *Monitor) IsConfirmedDegraded() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.confirmedDegraded
}

// ShouldNotify checks whether a notification for the given event type is allowed
// (not suppressed by cooldown). Returns true if notification should be sent.
// Flapping and stabilized share a cooldown to prevent rapid cycling between them.
func (m *Monitor) ShouldNotify(eventType string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.cooldownMinutes <= 0 {
		return true
	}
	cooldown := time.Duration(m.cooldownMinutes) * time.Minute

	// Flapping and stabilized share a cooldown — check whichever fired most recently
	if eventType == "flapping" || eventType == "stabilized" {
		var latest time.Time
		if t, ok := m.lastNotifiedAt["flapping"]; ok && t.After(latest) {
			latest = t
		}
		if t, ok := m.lastNotifiedAt["stabilized"]; ok && t.After(latest) {
			latest = t
		}
		if latest.IsZero() {
			return true
		}
		return time.Since(latest) >= cooldown
	}

	lastTime, exists := m.lastNotifiedAt[eventType]
	if !exists {
		return true
	}
	return time.Since(lastTime) >= cooldown
}

// MarkNotified records the current time as when a notification was sent for the given event type.
func (m *Monitor) MarkNotified(eventType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lastNotifiedAt == nil {
		m.lastNotifiedAt = make(map[string]time.Time)
	}
	m.lastNotifiedAt[eventType] = time.Now()
}

// ComputeFlapping evaluates the monitor's recent history for rapid state oscillation.
// Returns (isFlapping, changed) where changed is true if the flapping state transitioned.
func (m *Monitor) ComputeFlapping() (isFlapping bool, changed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.flapDetectionEnabled || len(m.history) < 3 {
		if m.isFlapping {
			m.isFlapping = false
			return false, true // stopped flapping
		}
		return false, false
	}

	// Count state transitions in the last N checks
	window := m.flapWindowChecks
	if window > len(m.history) {
		window = len(m.history)
	}
	start := len(m.history) - window

	transitions := 0
	for i := start + 1; i < len(m.history); i++ {
		prev := m.history[i-1]
		curr := m.history[i]
		// A transition is any change in up/down OR degraded state
		if prev.IsUp != curr.IsUp || prev.IsDegraded != curr.IsDegraded {
			transitions++
		}
	}

	// Calculate transition percentage (transitions / possible transitions * 100)
	possibleTransitions := window - 1
	if possibleTransitions <= 0 {
		if m.isFlapping {
			m.isFlapping = false
			return false, true
		}
		return false, false
	}

	transitionPercent := (transitions * 100) / possibleTransitions
	wasFlapping := m.isFlapping

	if transitionPercent >= m.flapThresholdPercent {
		// Grace period: don't re-enter flapping within cooldown window of stabilization.
		// This prevents rapid flapping→stabilized→flapping cycling.
		if !wasFlapping && !m.flapStabilizedAt.IsZero() {
			grace := time.Duration(m.cooldownMinutes) * time.Minute
			if grace < 5*time.Minute {
				grace = 5 * time.Minute // minimum 5-minute grace period
			}
			if time.Since(m.flapStabilizedAt) < grace {
				return false, false // still in grace period, suppress re-entry
			}
		}
		m.isFlapping = true
	} else {
		// Use hysteresis: only stop flapping at a lower threshold (80% of start threshold)
		stopThreshold := (m.flapThresholdPercent * 80) / 100
		if transitionPercent <= stopThreshold {
			m.isFlapping = false
		}
	}

	if m.isFlapping != wasFlapping {
		if !m.isFlapping {
			// Record when we stopped flapping for grace period
			m.flapStabilizedAt = time.Now()
		}
		return m.isFlapping, true
	}
	return m.isFlapping, false
}

// IsFlapping returns the current flapping state.
func (m *Monitor) IsFlapping() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isFlapping
}

// GetLatencyThreshold returns the effective latency threshold for this monitor.
func (m *Monitor) GetLatencyThreshold() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.latencyThreshold
}

// SetLatencyThreshold sets the effective latency threshold for this monitor.
func (m *Monitor) SetLatencyThreshold(v int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latencyThreshold = v
}

// IncrementRecovery increments the consecutive up counter during recovery confirmation.
// Returns true if the recovery confirmation threshold is met.
func (m *Monitor) IncrementRecovery() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.consecutiveUpCount++
	return m.consecutiveUpCount >= m.recoveryConfirmationChecks
}

// ResetRecovery resets the consecutive up counter (called when a failure occurs during recovery).
func (m *Monitor) ResetRecovery() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.consecutiveUpCount = 0
}

// HydrateConfirmationState scans the loaded history to restore confirmation counters
// so monitors already in a confirmed DOWN/DEGRADED state are correctly recognized on startup.
func (m *Monitor) HydrateConfirmationState() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.history) == 0 {
		return
	}

	// Count consecutive failures from the end of history
	downCount := 0
	degCount := 0

	for i := len(m.history) - 1; i >= 0; i-- {
		s := m.history[i]
		if !s.IsUp {
			downCount++
		} else {
			break
		}
	}

	// If not down, check degraded from end
	if downCount == 0 {
		for i := len(m.history) - 1; i >= 0; i-- {
			s := m.history[i]
			if s.IsUp && s.IsDegraded {
				degCount++
			} else {
				break
			}
		}
	}

	m.consecutiveDownCount = downCount
	m.confirmedDown = downCount >= m.confirmationThreshold

	m.consecutiveDegCount = degCount
	m.confirmedDegraded = degCount >= m.confirmationThreshold
}
