package uptime

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/notifications"
)

type Job struct {
	MonitorID string
	URL       string
}

type CheckResult struct {
	MonitorID  string
	URL        string
	Status     bool
	Latency    int64
	Timestamp  time.Time
	StatusCode int
	Error      string
	IsDegraded bool
	CertExpiry *time.Time // SSL certificate NotAfter (nil if not HTTPS or unavailable)
}

// SSL notification thresholds in days
var sslNotificationThresholds = []int{30, 14, 7, 1}

// sslThresholdState tracks which thresholds have been notified for a certificate
type sslThresholdState struct {
	CertExpiry time.Time    // Track cert expiry to detect renewal
	Notified   map[int]bool // threshold -> notified
}

type Manager struct {
	store    *db.Store
	monitors map[string]*Monitor // Map id -> Monitor
	mu       sync.RWMutex

	jobQueue    chan Job
	resultQueue chan CheckResult
	stopCh      chan struct{}
	wg          sync.WaitGroup

	latencyThreshold int64

	// Track SSL notification thresholds per monitor
	sslNotifiedThresholds map[string]*sslThresholdState

	// Cached notification timezone (loaded during Sync)
	notificationTimezone *time.Location

	// Active Maintenance Windows
	maintenanceWindows []db.Incident

	notifier *notifications.Service
}

const (
	WorkerCount = 50
	BatchSize   = 50
	BatchTime   = 2 * time.Second
)

func NewManager(store *db.Store) *Manager {
	m := &Manager{
		store:                 store,
		monitors:              make(map[string]*Monitor),
		maintenanceWindows:    make([]db.Incident, 0),
		jobQueue:              make(chan Job, 1000),         // Buffer for bursts
		resultQueue:           make(chan CheckResult, 1000), // Buffer for results
		stopCh:                make(chan struct{}),
		latencyThreshold:      1000, // Default
		sslNotifiedThresholds: make(map[string]*sslThresholdState),
		notificationTimezone:  time.UTC, // Default to UTC
		notifier:              notifications.NewService(store),
	}

	// Load settings
	if val, err := store.GetSetting("latency_threshold"); err == nil {
		if i, err := strconv.Atoi(val); err == nil {
			m.latencyThreshold = int64(i)
		}
	}

	return m
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

	// Start Retention Worker
	go m.retentionWorker()

	// Start Notification Service
	m.notifier.Start()

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

// Reset stops all monitors and clears the map. Used before DB reset.
func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, mon := range m.monitors {
		mon.Stop()
		delete(m.monitors, id)
	}
	m.sslNotifiedThresholds = make(map[string]*sslThresholdState)
}

func (m *Manager) worker() {
	defer m.wg.Done()

	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     30 * time.Second,
	}
	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: transport,
	}

	for job := range m.jobQueue {
		start := time.Now().UTC()
		resp, err := client.Get(job.URL)
		latency := time.Since(start).Milliseconds()

		isUp := true
		var errMsg string
		statusCode := 0
		var certExpiry *time.Time

		if err != nil {
			isUp = false
			errMsg = err.Error()
		} else {
			_ = resp.Body.Close()
			statusCode = resp.StatusCode
			if resp.StatusCode >= 400 {
				isUp = false
			}
			// Extract SSL certificate expiry for HTTPS URLs
			if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
				notAfter := resp.TLS.PeerCertificates[0].NotAfter
				certExpiry = &notAfter
			}
		}

		m.resultQueue <- CheckResult{
			MonitorID:  job.MonitorID,
			URL:        job.URL,
			Status:     isUp,
			Latency:    latency,
			Timestamp:  start,
			StatusCode: statusCode,
			Error:      errMsg,
			CertExpiry: certExpiry,
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
				active, _, hasHistory, lastDegraded := mon.GetLastStatus()

				// 2. Latency Threshold
				m.mu.RLock()
				threshold := m.latencyThreshold
				m.mu.RUnlock()

				// Check if monitor is in maintenance
				m.mu.RLock()
				isMaint := false
				now := time.Now().UTC()
				for _, w := range m.maintenanceWindows {
					if now.After(w.StartTime) && (w.EndTime == nil || now.Before(*w.EndTime)) {
						if w.AffectedGroups != "" {
							var groups []string
							_ = json.Unmarshal([]byte(w.AffectedGroups), &groups)
							for _, g := range groups {
								if g == mon.GetGroupID() {
									isMaint = true
									break
								}
							}
						}
					}
					if isMaint {
						break
					}
				}
				m.mu.RUnlock()

				isDegraded := res.Status && res.Latency > threshold
				res.IsDegraded = isDegraded // Update result for storage

				wasDegraded := active && lastDegraded

				message := "Monitor is down"
				if res.StatusCode > 0 {
					message += " (Status: " + strconv.Itoa(res.StatusCode) + ")"
				}

				if !hasHistory {
					// Handle Initial State
					if !res.Status {
						go func() {
							_ = m.store.CloseOutage(res.MonitorID)
							_ = m.store.CreateOutage(res.MonitorID, "down", message)
						}()
						go func() { _ = m.store.CreateEvent(res.MonitorID, "down", message) }()

						if !isMaint {
							m.notifier.Enqueue(notifications.NotificationEvent{
								MonitorID:   res.MonitorID,
								MonitorName: mon.GetName(),
								MonitorURL:  mon.GetTargetURL(),
								Type:        notifications.EventDown,
								Message:     message,
								Time:        res.Timestamp,
							})
						}
						log.Printf("Monitor %s is DOWN (Initial)", res.MonitorID)
					} else if isDegraded {
						go func() {
							_ = m.store.CloseOutage(res.MonitorID)
							_ = m.store.CreateOutage(res.MonitorID, "degraded", "High latency detected (>"+strconv.FormatInt(threshold, 10)+"ms)")
						}()
						go func() {
							_ = m.store.CreateEvent(res.MonitorID, "degraded", "High latency detected (>"+strconv.FormatInt(threshold, 10)+"ms)")
						}()
						if !isMaint {
							m.notifier.Enqueue(notifications.NotificationEvent{
								MonitorID:   res.MonitorID,
								MonitorName: mon.GetName(),
								MonitorURL:  mon.GetTargetURL(),
								Type:        notifications.EventDegraded,
								Message:     "High latency detected (> " + strconv.FormatInt(threshold, 10) + "ms)",
								Time:        res.Timestamp,
							})
						}
					}
				} else {
					// Handle Transitions
					if active && !res.Status {
						// UP -> DOWN
						go func() {
							_ = m.store.CloseOutage(res.MonitorID)
							_ = m.store.CreateOutage(res.MonitorID, "down", message)
						}()
						go func() { _ = m.store.CreateEvent(res.MonitorID, "down", message) }()

						if !isMaint {
							m.notifier.Enqueue(notifications.NotificationEvent{
								MonitorID:   res.MonitorID,
								MonitorName: mon.GetName(),
								MonitorURL:  mon.GetTargetURL(),
								Type:        notifications.EventDown,
								Message:     message,
								Time:        res.Timestamp,
							})
						}
						log.Printf("Monitor %s is DOWN: %s", res.MonitorID, message)
					} else if !active && res.Status {
						// DOWN -> UP
						go func() { _ = m.store.CloseOutage(res.MonitorID) }()
						go func() { _ = m.store.CreateEvent(res.MonitorID, "recovered", "Monitor recovered") }()
						if !isMaint {
							m.notifier.Enqueue(notifications.NotificationEvent{
								MonitorID:   res.MonitorID,
								MonitorName: mon.GetName(),
								MonitorURL:  mon.GetTargetURL(),
								Type:        notifications.EventUp,
								Message:     "Monitor Recovered",
								Time:        res.Timestamp,
							})
						}
						log.Printf("Monitor %s RECOVERED", res.MonitorID)
					}

					// Handle Degradation (Only if UP)
					if res.Status {
						if !wasDegraded && isDegraded {
							// Normal -> Degraded
							go func() {
								_ = m.store.CloseOutage(res.MonitorID)
								_ = m.store.CreateOutage(res.MonitorID, "degraded", "High latency detected (>"+strconv.FormatInt(threshold, 10)+"ms)")
							}()
							go func() {
								_ = m.store.CreateEvent(res.MonitorID, "degraded", "High latency detected (>"+strconv.FormatInt(threshold, 10)+"ms)")
							}()

							if !isMaint {
								m.notifier.Enqueue(notifications.NotificationEvent{
									MonitorID:   res.MonitorID,
									MonitorName: mon.GetName(),
									MonitorURL:  mon.GetTargetURL(),
									Type:        notifications.EventDegraded,
									Message:     "High latency detected (> " + strconv.FormatInt(threshold, 10) + "ms)",
									Time:        res.Timestamp,
								})
							}
						} else if wasDegraded && !isDegraded {
							// Degraded -> Normal (Optional: Log it? Or just let it be silent?)
							// For now, let's just log "recovered" from degradation?
							// Or maybe "Latency normalized"?
							go func() { _ = m.store.CloseOutage(res.MonitorID) }()
							go func() { _ = m.store.CreateEvent(res.MonitorID, "recovered", "Latency normalized") }()
						}
					}
				}

				// SSL Certificate Expiry Check - Threshold-based notifications at mid-day
				// Fixed thresholds: 30, 14, 7, 1 days
				if res.CertExpiry != nil && strings.HasPrefix(res.URL, "https") {
					daysUntilExpiry := int(time.Until(*res.CertExpiry).Hours() / 24)
					// Only process if within the maximum threshold (30 days)
					if daysUntilExpiry <= sslNotificationThresholds[0] {
						// Find the matching notification threshold (smallest applicable)
						// Thresholds: [30, 14, 7, 1] - iterate to find smallest t where daysUntilExpiry <= t
						matchedThreshold := -1
						for _, t := range sslNotificationThresholds {
							if daysUntilExpiry <= t {
								matchedThreshold = t // Keep updating to get smallest match
							}
						}

						// Check if we should notify (mid-day window + threshold not yet notified)
						shouldNotify := false
						if matchedThreshold > 0 {
							// Use cached notification timezone
							m.mu.RLock()
							loc := m.notificationTimezone
							m.mu.RUnlock()

							// Check if current time is in mid-day window (11:00 AM - 1:00 PM)
							nowLocal := time.Now().In(loc)
							hour := nowLocal.Hour()
							isMidDay := hour >= 11 && hour < 13

							if isMidDay {
								m.mu.Lock()
								state, exists := m.sslNotifiedThresholds[res.MonitorID]

								// Check if certificate was renewed (expiry date changed)
								if exists && !state.CertExpiry.Equal(*res.CertExpiry) {
									// Certificate renewed, reset all thresholds
									state = nil
									exists = false
								}

								if !exists {
									state = &sslThresholdState{
										CertExpiry: *res.CertExpiry,
										Notified:   make(map[int]bool),
									}
									m.sslNotifiedThresholds[res.MonitorID] = state
								}

								// Check if this threshold was already notified
								if !state.Notified[matchedThreshold] {
									state.Notified[matchedThreshold] = true
									shouldNotify = true
								}
								m.mu.Unlock()
							}
						}

						if shouldNotify {
							var msg string
							if daysUntilExpiry < 0 {
								msg = "SSL certificate expired " + strconv.Itoa(-daysUntilExpiry) + " days ago (" + res.CertExpiry.Format("2006-01-02") + ")"
							} else {
								msg = "SSL certificate expires in " + strconv.Itoa(daysUntilExpiry) + " days (" + res.CertExpiry.Format("2006-01-02") + ")"
							}
							go func() { _ = m.store.CreateEvent(res.MonitorID, "ssl_expiring", msg) }()

							if !isMaint {
								m.notifier.Enqueue(notifications.NotificationEvent{
									MonitorID:   res.MonitorID,
									MonitorName: mon.GetName(),
									MonitorURL:  mon.GetTargetURL(),
									Type:        notifications.EventSSLExpiring,
									Message:     msg,
									Time:        res.Timestamp,
								})
							}
							log.Printf("Monitor %s: SSL certificate expiring in %d days (threshold: %d)", res.MonitorID, daysUntilExpiry, matchedThreshold)
						}
					}
				}
			}

			// Update in-memory state
			m.updateMonitorState(res)

			// Add to batch for DB persistence
			statusStr := "down"
			if res.Status {
				statusStr = "up"
			}
			batch = append(batch, db.CheckResult{
				MonitorID:  res.MonitorID,
				Status:     statusStr,
				Latency:    res.Latency,
				Timestamp:  res.Timestamp,
				StatusCode: res.StatusCode,
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
		mon.RecordResult(res.Status, res.Latency, res.Timestamp, res.StatusCode, res.Error, res.IsDegraded)
	}
}

func (m *Manager) Sync() {
	dbMonitors, err := m.store.GetMonitors()
	if err != nil {
		log.Println("Error syncing monitors:", err)
		return
	}

	// Fetch Maintenance Windows
	var activeWindows []db.Incident
	incidents, err := m.store.GetIncidents(time.Time{})
	if err == nil {
		for _, i := range incidents {
			// Keep all scheduled/in-progress maintenance
			if i.Type == "maintenance" && i.Status != "completed" && i.Status != "resolved" {
				activeWindows = append(activeWindows, i)
			}
		}
	}

	// Load user timezone for notifications (from first/admin user)
	notifTZ := time.UTC
	if user, err := m.store.GetUser(1); err == nil && user.Timezone != "" {
		if loc, err := time.LoadLocation(user.Timezone); err == nil {
			notifTZ = loc
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Update cached settings
	m.notificationTimezone = notifTZ

	// Update maintenance windows
	m.maintenanceWindows = activeWindows

	activeIDs := make(map[string]bool)

	for _, dbM := range dbMonitors {
		activeIDs[dbM.ID] = true

		if !dbM.Active {
			// Ensure it's stopped
			if existing, exists := m.monitors[dbM.ID]; exists {
				existing.Stop()
				delete(m.monitors, dbM.ID)
				// Clean up SSL notification state so notifications will be re-sent when resumed
				delete(m.sslNotifiedThresholds, dbM.ID)
			}
			continue
		}

		// Determine interval
		intervalSec := dbM.Interval
		if intervalSec < 1 {
			intervalSec = 60
		}
		interval := time.Duration(intervalSec) * time.Second

		if existing, exists := m.monitors[dbM.ID]; exists {
			// Check for changes (URL or Interval)
			if existing.GetTargetURL() != dbM.URL || existing.GetInterval() != interval {
				log.Printf("Monitor %s config changed (Interval/URL). Restarting...", dbM.Name)
				existing.Stop()
				delete(m.monitors, dbM.ID)
			}
		}

		if _, exists := m.monitors[dbM.ID]; !exists {
			// Start new monitor
			mon := NewMonitor(dbM.ID, dbM.GroupID, dbM.Name, dbM.URL, interval, m.jobQueue, dbM.CreatedAt)

			// Hydrate history from DB
			checks, err := m.store.GetMonitorChecks(dbM.ID, 50)
			if err == nil {
				// Checks are returned DESC (Newest first).
				// We want to record them in order? RecordResult appends.
				// So we should iterate from end to start (Oldest to Newest).
				for i := len(checks) - 1; i >= 0; i-- {
					c := checks[i]
					isUp := c.Status == "up" // "up" or "down"
					isDegraded := isUp && c.Latency > m.latencyThreshold
					mon.RecordResult(isUp, c.Latency, c.Timestamp, c.StatusCode, "", isDegraded)
				}
			}

			go mon.Start()
			m.monitors[dbM.ID] = mon
			log.Printf("Scheduled monitor: %s (Interval: %ds)", dbM.Name, intervalSec)
		}
	}

	// Reconcile orphaned outages against current hydrated state
	if activeOutages, err := m.store.GetActiveOutages(); err == nil {
		for _, outage := range activeOutages {
			mon, exists := m.monitors[outage.MonitorID]
			if !exists {
				// Monitor is paused or deleted — preserve outage
				continue
			}
			isUp, _, hasHistory, lastDegraded := mon.GetLastStatus()
			if !hasHistory {
				continue
			}
			shouldClose := false
			if outage.Type == "down" && isUp {
				shouldClose = true
			} else if outage.Type == "degraded" && isUp && !lastDegraded {
				shouldClose = true
			}
			if shouldClose {
				if err := m.store.CloseOutage(outage.MonitorID); err != nil {
					log.Printf("Failed to close stale %s outage for %s: %v", outage.Type, outage.MonitorID, err)
				} else {
					log.Printf("Closed stale %s outage for monitor %s on startup reconciliation", outage.Type, outage.MonitorID)
				}
			}
		}
	}

	// Remove monitors that are no longer in DB
	for id, mon := range m.monitors {
		if !activeIDs[id] {
			mon.Stop()
			delete(m.monitors, id)
			delete(m.sslNotifiedThresholds, id)
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

// RemoveMonitor explicitly stops and removes a monitor.
// This is useful for immediate cleanup after deletion.
func (m *Manager) RemoveMonitor(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if mon, exists := m.monitors[id]; exists {
		mon.Stop()
		delete(m.monitors, id)
		delete(m.sslNotifiedThresholds, id)
		log.Printf("Explicitly stopped monitor: %s", id)
	}
}

func (m *Manager) SetLatencyThreshold(ms int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latencyThreshold = ms
}

func (m *Manager) GetLatencyThreshold() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.latencyThreshold
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

// IsGroupInMaintenance checks if a specific group is currently in an active maintenance window
func (m *Manager) IsGroupInMaintenance(groupID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now().UTC()
	for _, w := range m.maintenanceWindows {
		// Check time window
		if now.After(w.StartTime) && (w.EndTime == nil || now.Before(*w.EndTime)) {
			// Check affected groups
			if w.AffectedGroups != "" {
				var groups []string
				// Optimization: could cache unmarshal or simple string contains if confident
				if err := json.Unmarshal([]byte(w.AffectedGroups), &groups); err == nil {
					for _, g := range groups {
						if g == groupID {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

func (m *Manager) retentionWorker() {
	m.wg.Add(1)
	defer m.wg.Done()

	prune := func() {
		days := 30 // Default
		if val, err := m.store.GetSetting("data_retention_days"); err == nil {
			if i, err := strconv.Atoi(val); err == nil && i > 0 {
				days = i
			}
		}
		if err := m.store.PruneMonitorChecks(days); err != nil {
			log.Printf("Retention error: %v", err)
		}
	}

	// Run immediately
	prune()

	ticker := time.NewTicker(24 * time.Hour) // Run daily
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			prune()
		}
	}
}
