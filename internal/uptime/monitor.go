package uptime

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/clusteruptime/clusteruptime/internal/config"
)

type Status struct {
	Timestamp time.Time `json:"timestamp"`
	IsUp      bool      `json:"isUp"`
	Latency   int64     `json:"latencyMs"` // milliseconds
	Error     string    `json:"error,omitempty"`
}

type Monitor struct {
	cfg     config.Config
	history []Status
	mu      sync.RWMutex
	client  *http.Client
}

func NewMonitor(cfg config.Config) *Monitor {
	return &Monitor{
		cfg:     cfg,
		history: make([]Status, 0, 100),
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (m *Monitor) Start(ctx context.Context) {
	ticker := time.NewTicker(m.cfg.CheckInterval)
	defer ticker.Stop()

	// Initial check
	m.check()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.check()
		}
	}
}

func (m *Monitor) check() {
	if m.cfg.TargetURL == "" {
		return
	}

	start := time.Now()
	resp, err := m.client.Get(m.cfg.TargetURL)
	latency := time.Since(start).Milliseconds()

	status := Status{
		Timestamp: time.Now(),
		Latency:   latency,
		IsUp:      true,
	}

	if err != nil {
		status.IsUp = false
		status.Error = err.Error()
	} else {
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			status.IsUp = false
			status.Error = resp.Status
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Keep last 100 checks
	if len(m.history) >= 100 {
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
