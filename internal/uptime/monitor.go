package uptime

import (
	"sync"
	"time"
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
	jobQueue  chan<- Job

	// Notification fatigue state (protected by mu)
	confirmationThreshold int // effective threshold (resolved from per-monitor or global)
	cooldownMinutes       int // effective cooldown (resolved from per-monitor or global)

	consecutiveDownCount int  // consecutive failed checks
	consecutiveDegCount  int  // consecutive degraded checks
	confirmedDown        bool // threshold met for down
	confirmedDegraded    bool // threshold met for degraded

	lastNotifiedAt map[string]time.Time // per-event-type cooldown tracking
	isFlapping     bool                 // current flap state

	// Flap detection settings
	flapDetectionEnabled bool
	flapWindowChecks     int
	flapThresholdPercent int
}

// MonitorConfig holds per-monitor notification fatigue settings.
type MonitorConfig struct {
	ConfirmationThreshold int
	CooldownMinutes       int
	FlapDetectionEnabled  bool
	FlapWindowChecks      int
	FlapThresholdPercent  int
}

func NewMonitor(id, groupID, name, url string, interval time.Duration, jobQueue chan<- Job, createdAt time.Time) *Monitor {
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
		confirmationThreshold: 3,  // default
		cooldownMinutes:       30, // default
		lastNotifiedAt:        make(map[string]time.Time),
		flapDetectionEnabled:  true,
		flapWindowChecks:      21,
		flapThresholdPercent:  25,
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
	select {
	case m.jobQueue <- Job{MonitorID: m.id, URL: m.url}:
		// Scheduled
	default:
		// Queue full, skip this tick to avoid blocking scheduler
		// Log warning?
	}
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
func (m *Monitor) ShouldNotify(eventType string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.cooldownMinutes <= 0 {
		return true
	}
	lastTime, exists := m.lastNotifiedAt[eventType]
	if !exists {
		return true
	}
	return time.Since(lastTime) >= time.Duration(m.cooldownMinutes)*time.Minute
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
		m.isFlapping = true
	} else {
		// Use hysteresis: only stop flapping at a lower threshold (80% of start threshold)
		stopThreshold := (m.flapThresholdPercent * 80) / 100
		if transitionPercent <= stopThreshold {
			m.isFlapping = false
		}
	}

	return m.isFlapping, m.isFlapping != wasFlapping
}

// IsFlapping returns the current flapping state.
func (m *Monitor) IsFlapping() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isFlapping
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
