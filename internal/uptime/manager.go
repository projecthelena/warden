package uptime

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/clusteruptime/clusteruptime/internal/db"
)

type Job struct {
	MonitorID string
	URL       string
}

type CheckResult struct {
	MonitorID string
	URL       string
	Status    bool
	Latency   int64
	Timestamp time.Time
}

type Manager struct {
	store    *db.Store
	monitors map[string]*Monitor // Map id -> Monitor
	mu       sync.RWMutex

	jobQueue    chan Job
	resultQueue chan CheckResult
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

const (
	WorkerCount = 50
	BatchSize   = 50
	BatchTime   = 2 * time.Second
)

func NewManager(store *db.Store) *Manager {
	return &Manager{
		store:       store,
		monitors:    make(map[string]*Monitor),
		jobQueue:    make(chan Job, 1000),         // Buffer for bursts
		resultQueue: make(chan CheckResult, 1000), // Buffer for results
		stopCh:      make(chan struct{}),
	}
}

func (m *Manager) Start() {
	// Start Workers
	for i := 0; i < WorkerCount; i++ {
		m.wg.Add(1)
		go m.worker()
	}

	// Start Result Processor (Batch Writer)
	m.wg.Add(1)
	go m.resultProcessor()

	// Initial Sync
	m.Sync()

	// Periodic Sync (e.g. every 10 seconds to catch DB changes if no explicit signal)
	// For this MVP, we can also expose a Sync method to the API handler.
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.Sync()
			}
		}
	}()
}

func (m *Manager) Stop() {
	close(m.stopCh)
	// Stop monitors (producers)
	m.mu.Lock()
	for _, mon := range m.monitors {
		mon.Stop()
	}
	m.mu.Unlock()

	close(m.jobQueue)
	// Wait for workers to finish
	// m.wg.Wait() // Optional: strictly wait or just let app exit
}

func (m *Manager) worker() {
	defer m.wg.Done()
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	for job := range m.jobQueue {
		start := time.Now()
		resp, err := client.Get(job.URL)
		latency := time.Since(start).Milliseconds()

		isUp := true
		if err != nil {
			isUp = false
		} else {
			resp.Body.Close()
			if resp.StatusCode >= 400 {
				isUp = false
			}
		}

		m.resultQueue <- CheckResult{
			MonitorID: job.MonitorID,
			URL:       job.URL,
			Status:    isUp,
			Latency:   latency,
			Timestamp: start,
		}
	}
}

func (m *Manager) resultProcessor() {
	defer m.wg.Done()

	var batch []db.CheckResult
	timer := time.NewTicker(BatchTime)
	defer timer.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := m.store.BatchInsertChecks(batch); err != nil {
			log.Printf("Error capturing batch stats: %v", err)
		}
		batch = nil
	}

	for {
		select {
		case <-m.stopCh:
			flush()
			return
		case <-timer.C:
			flush()
		case res := <-m.resultQueue:
			// 1. Detect Events (State Change)
			m.mu.RLock()
			mon, exists := m.monitors[res.MonitorID]
			m.mu.RUnlock()

			if exists {
				active, lastLatency, hasHistory := mon.GetLastStatus()

				// Only detect events if we have previous history (don't alert on first run/boot unless we Hydrated)
				// Since we Hydrate from DB, hasHistory should be true if it was running.
				// If it's a brand new monitor, hasHistory is false -> No event (correct).

				if hasHistory {
					// 1. UP <-> DOWN
					if active && !res.Status {
						// DOWN Event
						go m.store.CreateEvent(res.MonitorID, "down", "Monitor is down")
						log.Printf("Monitor %s is DOWN", res.MonitorID)
					} else if !active && res.Status {
						// RECOVERED Event
						go m.store.CreateEvent(res.MonitorID, "recovered", "Monitor recovered")
						log.Printf("Monitor %s RECOVERED", res.MonitorID)
					}

					// 2. Latency Degradation (Simple Threshold: 500ms)
					if res.Status && res.Latency > 500 {
						if lastLatency <= 500 {
							// New Degradation
							go m.store.CreateEvent(res.MonitorID, "degraded", "High latency detected (>500ms)")
						}
					}
				}
			}

			// 2. Update In-Memory State
			m.updateMonitorState(res)

			// 2. Add to Batch (for DB persistence)
			statusStr := "down"
			if res.Status {
				statusStr = "up"
			}
			batch = append(batch, db.CheckResult{
				MonitorID: res.MonitorID,
				Status:    statusStr,
				Latency:   res.Latency,
				Timestamp: res.Timestamp,
			})

			if len(batch) >= BatchSize {
				flush()
			}
		}
	}
}

func (m *Manager) updateMonitorState(res CheckResult) {
	m.mu.RLock()
	mon, exists := m.monitors[res.MonitorID]
	m.mu.RUnlock()

	if exists {
		mon.RecordResult(res.Status, res.Latency, res.Timestamp)
	}
}

func (m *Manager) Sync() {
	dbMonitors, err := m.store.GetMonitors()
	if err != nil {
		log.Println("Error syncing monitors:", err)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	activeIDs := make(map[string]bool)

	for _, dbM := range dbMonitors {
		activeIDs[dbM.ID] = true

		if !dbM.Active {
			// Ensure it's stopped
			if existing, exists := m.monitors[dbM.ID]; exists {
				existing.Stop()
				delete(m.monitors, dbM.ID)
			}
			continue
		}

		if _, exists := m.monitors[dbM.ID]; !exists {
			// Start new monitor passing the JobQueue
			mon := NewMonitor(dbM.ID, dbM.URL, 10*time.Second, m.jobQueue) // Default 10s interval for now

			// Hydrate history from DB
			checks, err := m.store.GetMonitorChecks(dbM.ID, 50)
			if err == nil {
				// Checks are returned DESC (Newest first).
				// We want to record them in order? RecordResult appends.
				// So we should iterate from end to start (Oldest to Newest).
				for i := len(checks) - 1; i >= 0; i-- {
					c := checks[i]
					isUp := c.Status == "up" // "up" or "down"
					mon.RecordResult(isUp, c.Latency, c.Timestamp)
				}
			}

			go mon.Start()
			m.monitors[dbM.ID] = mon
			log.Printf("Scheduled monitor: %s", dbM.Name)
		}
	}

	// Remove monitors that are no longer in DB
	for id, mon := range m.monitors {
		if !activeIDs[id] {
			mon.Stop()
			delete(m.monitors, id)
			log.Printf("Stopped monitor: %s", id)
		}
	}
}

// GetMonitor returns a specific monitor instance
func (m *Manager) GetMonitor(id string) *Monitor {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.monitors[id]
}

// GetAll returns all running monitors
func (m *Manager) GetAll() map[string]*Monitor {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Return shallow copy of map to avoid race on iteration?
	// Or just return atomic snapshot.
	res := make(map[string]*Monitor)
	for k, v := range m.monitors {
		res[k] = v
	}
	return res
}
