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
}

func NewMonitor(id, groupID, name, url string, interval time.Duration, jobQueue chan<- Job, createdAt time.Time) *Monitor {
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	return &Monitor{
		id:        id,
		groupID:   groupID,
		name:      name,
		url:       url,
		interval:  interval,
		createdAt: createdAt,
		history:   make([]Status, 0, 50), // Keep last 50 in memory
		stopCh:    make(chan struct{}),
		jobQueue:  jobQueue,
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
