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
	id       string
	groupID  string
	name     string
	url      string
	interval time.Duration
	history  []Status
	mu       sync.RWMutex
	stopCh   chan struct{}
	jobQueue chan<- Job
}

func NewMonitor(id, groupID, name, url string, interval time.Duration, jobQueue chan<- Job) *Monitor {
	return &Monitor{
		id:       id,
		groupID:  groupID,
		name:     name,
		url:      url,
		interval: interval,
		history:  make([]Status, 0, 50), // Keep last 50 in memory
		stopCh:   make(chan struct{}),
		jobQueue: jobQueue,
	}
}

func (m *Monitor) Start() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	// Initial check
	m.schedule()

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
	close(m.stopCh)
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
