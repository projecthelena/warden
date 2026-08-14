package uptime

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/notifications"
)

type Job struct {
	MonitorID     string
	Type          string // check type; empty means http
	URL           string
	RequestConfig *db.RequestConfig
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
	// NotRun marks a check that never reached the target: an address Warden cannot use,
	// or a permission it lacks. Still down, but reporting it as "down" sends the
	// operator hunting a network problem that isn't there, so the reason replaces the
	// usual message.
	NotRun          bool
	CertExpiry      *time.Time // SSL certificate NotAfter (nil if not HTTPS or unavailable)
	ResponseBody    string     // Truncated to ResponseBodyMaxBytes; only populated on failed/degraded checks
	ResponseHeaders string     // JSON-encoded filtered headers; only populated on failed/degraded checks
}

// ResponseBodyMaxBytes caps how much of a failed response body is captured for diagnostics.
const ResponseBodyMaxBytes = 2048

// captureHeaderAllowlist lists response headers worth storing for diagnostics.
// Sensitive headers (set-cookie, authorization, etc.) are deliberately excluded.
var captureHeaderAllowlist = map[string]bool{
	"content-type":     true,
	"content-length":   true,
	"content-encoding": true,
	"cache-control":    true,
	"server":           true,
	"date":             true,
	"location":         true,
	"retry-after":      true,
	"x-request-id":     true,
	"x-correlation-id": true,
	"x-trace-id":       true,
	"x-served-by":      true,
	"cf-ray":           true,
	"via":              true,
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

	// Notification event filter (per-event-type toggles)
	eventFilter NotificationEventFilter

	// Digest configuration
	digestEnabled    bool
	digestTime       string // HH:MM
	digestEventTypes map[string]bool

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
		eventFilter: NotificationEventFilter{
			DownEnabled:        true,
			UpEnabled:          true,
			DegradedEnabled:    true,
			FlappingEnabled:    true,
			StabilizedEnabled:  true,
			SSLExpiringEnabled: true,
		},
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

	// Start Digest Worker
	go m.digestWorker()

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
		DialContext:         dialContext,
	}

	for job := range m.jobQueue {
		m.resultQueue <- runCheck(job, transport)
	}
}

// eventDetailsFromResult builds the diagnostic payload that we persist alongside a monitor
// event so the drill-down view can show what the check actually returned. We keep the heavy
// fields (response body, headers) only when the check itself failed.
func eventDetailsFromResult(res CheckResult) *db.EventDetails {
	d := &db.EventDetails{
		StatusCode:   res.StatusCode,
		Latency:      res.Latency,
		ErrorMessage: res.Error,
	}
	if !res.Status {
		d.ResponseBody = res.ResponseBody
		d.ResponseHeaders = res.ResponseHeaders
	}
	return d
}

// readLimitedBody reads up to ResponseBodyMaxBytes of an HTTP response body and returns it as a
// UTF-8 safe string. Used only for diagnostic capture on failed checks.
func readLimitedBody(body io.ReadCloser) string {
	if body == nil {
		return ""
	}
	buf := make([]byte, ResponseBodyMaxBytes)
	n, _ := io.ReadFull(io.LimitReader(body, ResponseBodyMaxBytes), buf)
	if n == 0 {
		return ""
	}
	out := buf[:n]
	if !utf8.Valid(out) {
		return strconv.Quote(string(out))
	}
	return string(out)
}

// encodeAllowedHeaders serializes the subset of response headers listed in captureHeaderAllowlist
// to a compact JSON object. Sensitive headers (cookies, auth) are dropped to avoid leaking
// session material into the digest archive.
func encodeAllowedHeaders(h http.Header) string {
	if len(h) == 0 {
		return ""
	}
	filtered := make(map[string]string, len(captureHeaderAllowlist))
	for k, vs := range h {
		if len(vs) == 0 {
			continue
		}
		if captureHeaderAllowlist[strings.ToLower(k)] {
			filtered[k] = strings.Join(vs, ", ")
		}
	}
	if len(filtered) == 0 {
		return ""
	}
	b, err := json.Marshal(filtered)
	if err != nil {
		return ""
	}
	return string(b)
}

// isAcceptedStatus checks if a status code matches the accepted status code specification.
// Spec format: "200-299,301,302" — comma-separated codes or ranges.
func isAcceptedStatus(code int, spec string) bool {
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if dashIdx := strings.Index(part, "-"); dashIdx >= 0 {
			lo, err1 := strconv.Atoi(strings.TrimSpace(part[:dashIdx]))
			hi, err2 := strconv.Atoi(strings.TrimSpace(part[dashIdx+1:]))
			if err1 == nil && err2 == nil && code >= lo && code <= hi {
				return true
			}
		} else {
			val, err := strconv.Atoi(part)
			if err == nil && code == val {
				return true
			}
		}
	}
	return false
}

// requestConfigChanged compares two RequestConfig pointers for semantic equality.
func requestConfigChanged(a, b *db.RequestConfig) bool {
	if a == nil && b == nil {
		return false
	}
	if a == nil || b == nil {
		return true
	}
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return string(aJSON) != string(bJSON)
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

			// Load event filter snapshot
			m.mu.RLock()
			eventFilter := m.eventFilter
			m.mu.RUnlock()

			if exists {
				active, _, hasHistory, lastDegraded := mon.GetLastStatus()

				// 2. Latency Threshold (per-monitor override or global default)
				threshold := mon.GetLatencyThreshold()

				// Check if monitor is in maintenance
				isMaint := m.isMonitorInMaintenance(mon.GetGroupID())

				isDegraded := res.Status && res.Latency > threshold
				res.IsDegraded = isDegraded // Update result for storage

				wasDegraded := active && lastDegraded

				message := "Monitor is down"
				if res.NotRun && res.Error != "" {
					message = res.Error
				} else if res.StatusCode > 0 {
					message += " (Status: " + strconv.Itoa(res.StatusCode) + ")"
				}

				degradedMsg := "High latency detected (>" + strconv.FormatInt(threshold, 10) + "ms)"

				if !hasHistory {
					// Handle Initial State — use confirmation logic
					if !res.Status {
						mon.ResetRecovery()
						// Record the event in DB immediately
						eventDetails := eventDetailsFromResult(res)
						go func() { _ = m.store.CreateEventWithDetails(res.MonitorID, "down", message, eventDetails) }()

						confirmed := mon.IncrementDown()
						if confirmed {
							go func() {
								_ = m.store.CloseOutage(res.MonitorID)
								_ = m.store.CreateOutage(res.MonitorID, "down", message)
							}()
							if !isMaint && !mon.IsFlapping() && mon.ShouldNotify("down") && eventFilter.IsEnabled("down") {
								m.enqueueOrDigest(notifications.NotificationEvent{
									MonitorID:   res.MonitorID,
									MonitorName: mon.GetName(),
									MonitorURL:  mon.GetTargetURL(),
									Type:        notifications.EventDown,
									Message:     message,
									Time:        res.Timestamp,
								})
								mon.MarkNotified("down")
							}
							log.Printf("Monitor %s is DOWN (confirmed)", res.MonitorID)
						}
					} else if isDegraded {
						degDetails := eventDetailsFromResult(res)
						go func() { _ = m.store.CreateEventWithDetails(res.MonitorID, "degraded", degradedMsg, degDetails) }()

						confirmed := mon.IncrementDegraded()
						if confirmed {
							go func() {
								_ = m.store.CloseOutage(res.MonitorID)
								_ = m.store.CreateOutage(res.MonitorID, "degraded", degradedMsg)
							}()
							if !isMaint && !mon.IsFlapping() && mon.ShouldNotify("degraded") && eventFilter.IsEnabled("degraded") {
								m.enqueueOrDigest(notifications.NotificationEvent{
									MonitorID:   res.MonitorID,
									MonitorName: mon.GetName(),
									MonitorURL:  mon.GetTargetURL(),
									Type:        notifications.EventDegraded,
									Message:     degradedMsg,
									Time:        res.Timestamp,
								})
								mon.MarkNotified("degraded")
							}
						}
					}
				} else {
					// Handle Transitions with confirmation logic
					if !res.Status {
						// Check is DOWN — increment counter
						mon.ResetDegraded() // can't be degraded if down
						mon.ResetRecovery() // reset recovery confirmation
						downDetails := eventDetailsFromResult(res)
						go func() { _ = m.store.CreateEventWithDetails(res.MonitorID, "down", message, downDetails) }()

						confirmed := mon.IncrementDown()
						if confirmed {
							// Threshold met — create outage and notify
							go func() {
								_ = m.store.CloseOutage(res.MonitorID)
								_ = m.store.CreateOutage(res.MonitorID, "down", message)
							}()
							if !isMaint && !mon.IsFlapping() && mon.ShouldNotify("down") && eventFilter.IsEnabled("down") {
								m.enqueueOrDigest(notifications.NotificationEvent{
									MonitorID:   res.MonitorID,
									MonitorName: mon.GetName(),
									MonitorURL:  mon.GetTargetURL(),
									Type:        notifications.EventDown,
									Message:     message,
									Time:        res.Timestamp,
								})
								mon.MarkNotified("down")
							}
							log.Printf("Monitor %s is DOWN (confirmed): %s", res.MonitorID, message)
						}
					} else {
						// Check is UP
						// Recovery from confirmed down?
						if mon.IsConfirmedDown() {
							recoveryConfirmed := mon.IncrementRecovery()
							if recoveryConfirmed {
								mon.ResetDown()
								mon.ResetRecovery()
								go func() { _ = m.store.CloseOutage(res.MonitorID) }()
								recDetails := eventDetailsFromResult(res)
								go func() {
									_ = m.store.CreateEventWithDetails(res.MonitorID, "recovered", "Monitor recovered", recDetails)
								}()
								// Recovery notifications always send immediately (no cooldown)
								if !isMaint && !mon.IsFlapping() && eventFilter.IsEnabled("up") {
									m.enqueueOrDigest(notifications.NotificationEvent{
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
						} else {
							// Not confirmed down — just reset the counter
							mon.ResetDown()
						}

						// Handle Degradation (only if not still waiting for recovery confirmation)
						if !mon.IsConfirmedDown() {
							if isDegraded {
								degDetails := eventDetailsFromResult(res)
								go func() { _ = m.store.CreateEventWithDetails(res.MonitorID, "degraded", degradedMsg, degDetails) }()

								confirmed := mon.IncrementDegraded()
								if confirmed {
									go func() {
										_ = m.store.CloseOutage(res.MonitorID)
										_ = m.store.CreateOutage(res.MonitorID, "degraded", degradedMsg)
									}()
									if !isMaint && !mon.IsFlapping() && mon.ShouldNotify("degraded") && eventFilter.IsEnabled("degraded") {
										m.enqueueOrDigest(notifications.NotificationEvent{
											MonitorID:   res.MonitorID,
											MonitorName: mon.GetName(),
											MonitorURL:  mon.GetTargetURL(),
											Type:        notifications.EventDegraded,
											Message:     degradedMsg,
											Time:        res.Timestamp,
										})
										mon.MarkNotified("degraded")
									}
								}
							} else if wasDegraded {
								// Degraded -> Normal
								wasConfirmedDeg := mon.ResetDegraded()
								if wasConfirmedDeg {
									go func() { _ = m.store.CloseOutage(res.MonitorID) }()
									recDetails := eventDetailsFromResult(res)
									go func() {
										_ = m.store.CreateEventWithDetails(res.MonitorID, "recovered", "Latency normalized", recDetails)
									}()
									// Recovery notifications always send immediately (no cooldown)
									if !isMaint && !mon.IsFlapping() && eventFilter.IsEnabled("up") {
										m.enqueueOrDigest(notifications.NotificationEvent{
											MonitorID:   res.MonitorID,
											MonitorName: mon.GetName(),
											MonitorURL:  mon.GetTargetURL(),
											Type:        notifications.EventUp,
											Message:     "Latency normalized",
											Time:        res.Timestamp,
										})
									}
									log.Printf("Monitor %s RECOVERED from degraded", res.MonitorID)
								}
							} else {
								// Normal -> Normal: reset degraded counter
								mon.ResetDegraded()
							}
						}
					}
				}

				// SSL Certificate Expiry Check
				m.processSSLCheck(res, mon, isMaint)

				// Flap Detection (after recording result, so history is up to date)
				// We process this after updateMonitorState below
			}

			// Update in-memory state
			m.updateMonitorState(res)

			// Flap detection (after history is updated)
			if exists {
				m.mu.RLock()
				mon := m.monitors[res.MonitorID]
				m.mu.RUnlock()
				if mon != nil {
					isMaint := m.isMonitorInMaintenance(mon.GetGroupID())
					isFlapping, changed := mon.ComputeFlapping()
					if changed && !isMaint {
						if isFlapping {
							go func() { _ = m.store.CreateEvent(res.MonitorID, "flapping", "Monitor is flapping between states") }()
							if mon.ShouldNotify("flapping") && eventFilter.IsEnabled("flapping") {
								m.enqueueOrDigest(notifications.NotificationEvent{
									MonitorID:   res.MonitorID,
									MonitorName: mon.GetName(),
									MonitorURL:  mon.GetTargetURL(),
									Type:        notifications.EventFlapping,
									Message:     "Monitor is flapping between states",
									Time:        res.Timestamp,
								})
								mon.MarkNotified("flapping")
							}
							log.Printf("Monitor %s is FLAPPING", res.MonitorID)
						} else {
							go func() { _ = m.store.CreateEvent(res.MonitorID, "stabilized", "Monitor has stabilized") }()
							if mon.ShouldNotify("stabilized") && eventFilter.IsEnabled("stabilized") {
								m.enqueueOrDigest(notifications.NotificationEvent{
									MonitorID:   res.MonitorID,
									MonitorName: mon.GetName(),
									MonitorURL:  mon.GetTargetURL(),
									Type:        notifications.EventStabilized,
									Message:     "Monitor has stabilized",
									Time:        res.Timestamp,
								})
								mon.MarkNotified("stabilized")
							}
							log.Printf("Monitor %s STABILIZED", res.MonitorID)
						}
					}
				}
			}

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

// isMonitorInMaintenance checks if a monitor's group is in an active maintenance window.
func (m *Manager) isMonitorInMaintenance(groupID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	now := time.Now().UTC()
	for _, w := range m.maintenanceWindows {
		if now.After(w.StartTime) && (w.EndTime == nil || now.Before(*w.EndTime)) {
			if w.AffectedGroups != "" {
				var groups []string
				_ = json.Unmarshal([]byte(w.AffectedGroups), &groups)
				for _, g := range groups {
					if g == groupID {
						return true
					}
				}
			}
		}
	}
	return false
}

// processSSLCheck handles SSL certificate expiry checking and notifications.
func (m *Manager) processSSLCheck(res CheckResult, mon *Monitor, isMaint bool) {
	if res.CertExpiry == nil || !strings.HasPrefix(res.URL, "https") {
		return
	}
	daysUntilExpiry := int(time.Until(*res.CertExpiry).Hours() / 24)
	if daysUntilExpiry > sslNotificationThresholds[0] {
		return
	}

	matchedThreshold := -1
	for _, t := range sslNotificationThresholds {
		if daysUntilExpiry <= t {
			matchedThreshold = t
		}
	}

	shouldNotify := false
	if matchedThreshold > 0 {
		m.mu.RLock()
		loc := m.notificationTimezone
		m.mu.RUnlock()

		nowLocal := time.Now().In(loc)
		hour := nowLocal.Hour()
		isMidDay := hour >= 11 && hour < 13

		if isMidDay {
			m.mu.Lock()
			state, exists := m.sslNotifiedThresholds[res.MonitorID]

			if exists && !state.CertExpiry.Equal(*res.CertExpiry) {
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

		m.mu.RLock()
		filter := m.eventFilter
		m.mu.RUnlock()

		if !isMaint && filter.IsEnabled("ssl_expiring") {
			m.enqueueOrDigest(notifications.NotificationEvent{
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
		} else {
			log.Printf("Digest: invalid timezone %q for user 1, falling back to UTC: %v", user.Timezone, err)
		}
	}

	// Load global notification fatigue settings
	globalCfg := m.loadNotificationConfig()

	// Load event filter and digest config
	eventFilter := m.loadEventFilter()
	digestEnabled, digestTime, digestEventTypes := m.loadDigestConfig()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Update cached settings
	m.notificationTimezone = notifTZ
	m.eventFilter = eventFilter
	m.digestEnabled = digestEnabled
	m.digestTime = digestTime
	m.digestEventTypes = digestEventTypes

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

		// Resolve per-monitor config (override global defaults)
		cfg := globalCfg
		if dbM.ConfirmationThreshold != nil {
			cfg.ConfirmationThreshold = *dbM.ConfirmationThreshold
		}
		if dbM.NotificationCooldownMin != nil {
			cfg.CooldownMinutes = *dbM.NotificationCooldownMin
		}

		// Determine interval
		intervalSec := dbM.Interval
		if intervalSec < 1 {
			intervalSec = 60
		}
		interval := time.Duration(intervalSec) * time.Second

		// Resolve per-monitor latency threshold
		monLatencyThresh := m.latencyThreshold // global default
		if dbM.LatencyThreshold != nil {
			monLatencyThresh = int64(*dbM.LatencyThreshold)
		}

		if existing, exists := m.monitors[dbM.ID]; exists {
			// Always apply latest config to existing monitors
			existing.ApplyConfig(cfg)
			existing.SetLatencyThreshold(monLatencyThresh)

			// Check for changes (URL, Type, Interval, or RequestConfig)
			needRestart := existing.GetTargetURL() != dbM.URL ||
				existing.GetType() != db.NormalizeMonitorType(dbM.Type) ||
				existing.GetInterval() != interval
			if !needRestart && requestConfigChanged(existing.GetRequestConfig(), dbM.RequestConfig) {
				needRestart = true
			}
			if needRestart {
				log.Printf("Monitor %s config changed. Restarting...", dbM.Name)
				existing.Stop()
				delete(m.monitors, dbM.ID)
			}
		}

		if _, exists := m.monitors[dbM.ID]; !exists {
			// Start new monitor
			mon := NewMonitor(dbM.ID, dbM.Type, dbM.GroupID, dbM.Name, dbM.URL, interval, m.jobQueue, dbM.CreatedAt, dbM.RequestConfig)
			mon.ApplyConfig(cfg)
			mon.SetLatencyThreshold(monLatencyThresh)

			// Hydrate history from DB
			checks, err := m.store.GetMonitorChecks(dbM.ID, 50)
			if err == nil {
				// Checks are returned DESC (Newest first).
				// We want to record them in order? RecordResult appends.
				// So we should iterate from end to start (Oldest to Newest).
				for i := len(checks) - 1; i >= 0; i-- {
					c := checks[i]
					isUp := c.Status == "up" // "up" or "down"
					isDegraded := isUp && c.Latency > mon.GetLatencyThreshold()
					mon.RecordResult(isUp, c.Latency, c.Timestamp, c.StatusCode, "", isDegraded)
				}
			}

			// Hydrate confirmation state from history
			mon.HydrateConfirmationState()

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

// loadNotificationConfig reads global notification fatigue settings from the database.
func (m *Manager) loadNotificationConfig() MonitorConfig {
	cfg := MonitorConfig{
		ConfirmationThreshold:      3,
		CooldownMinutes:            30,
		FlapDetectionEnabled:       true,
		FlapWindowChecks:           21,
		FlapThresholdPercent:       25,
		RecoveryConfirmationChecks: 1,
	}

	if val, err := m.store.GetSetting("notification.confirmation_threshold"); err == nil {
		if i, err := strconv.Atoi(val); err == nil && i >= 1 {
			cfg.ConfirmationThreshold = i
		}
	}
	if val, err := m.store.GetSetting("notification.cooldown_minutes"); err == nil {
		if i, err := strconv.Atoi(val); err == nil && i >= 0 {
			cfg.CooldownMinutes = i
		}
	}
	if val, err := m.store.GetSetting("notification.flap_detection_enabled"); err == nil {
		cfg.FlapDetectionEnabled = val == "true"
	}
	if val, err := m.store.GetSetting("notification.flap_window_checks"); err == nil {
		if i, err := strconv.Atoi(val); err == nil && i >= 3 {
			cfg.FlapWindowChecks = i
		}
	}
	if val, err := m.store.GetSetting("notification.flap_threshold_percent"); err == nil {
		if i, err := strconv.Atoi(val); err == nil && i >= 1 && i <= 100 {
			cfg.FlapThresholdPercent = i
		}
	}
	if val, err := m.store.GetSetting("notification.recovery_confirmation_checks"); err == nil {
		if i, err := strconv.Atoi(val); err == nil && i >= 1 {
			cfg.RecoveryConfirmationChecks = i
		}
	}

	return cfg
}

// loadEventFilter reads per-event-type notification toggles from the database.
func (m *Manager) loadEventFilter() NotificationEventFilter {
	filter := NotificationEventFilter{
		DownEnabled:        true,
		UpEnabled:          true,
		DegradedEnabled:    true,
		FlappingEnabled:    true,
		StabilizedEnabled:  true,
		SSLExpiringEnabled: true,
	}

	if val, err := m.store.GetSetting("notification.event.down.enabled"); err == nil {
		filter.DownEnabled = val != "false"
	}
	if val, err := m.store.GetSetting("notification.event.up.enabled"); err == nil {
		filter.UpEnabled = val != "false"
	}
	if val, err := m.store.GetSetting("notification.event.degraded.enabled"); err == nil {
		filter.DegradedEnabled = val != "false"
	}
	if val, err := m.store.GetSetting("notification.event.flapping.enabled"); err == nil {
		filter.FlappingEnabled = val != "false"
	}
	if val, err := m.store.GetSetting("notification.event.stabilized.enabled"); err == nil {
		filter.StabilizedEnabled = val != "false"
	}
	if val, err := m.store.GetSetting("notification.event.ssl_expiring.enabled"); err == nil {
		filter.SSLExpiringEnabled = val != "false"
	}

	return filter
}

// loadDigestConfig reads daily digest settings from the database.
func (m *Manager) loadDigestConfig() (bool, string, map[string]bool) {
	enabled := false
	digestTime := "09:00"
	eventTypes := map[string]bool{
		"degraded":     true,
		"flapping":     true,
		"stabilized":   true,
		"ssl_expiring": true,
	}

	if val, err := m.store.GetSetting("notification.digest.enabled"); err == nil {
		enabled = val == "true"
	}
	if val, err := m.store.GetSetting("notification.digest.time"); err == nil && val != "" {
		digestTime = val
	}
	if val, err := m.store.GetSetting("notification.digest.event_types"); err == nil && val != "" {
		eventTypes = make(map[string]bool)
		for _, t := range strings.Split(val, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				eventTypes[t] = true
			}
		}
	}

	return enabled, digestTime, eventTypes
}

// shouldDigest checks if an event type should be routed to the digest queue.
func (m *Manager) shouldDigest(eventType string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.digestEnabled {
		return false
	}
	return m.digestEventTypes[eventType]
}

// enqueueOrDigest either sends a notification immediately or queues it for digest.
func (m *Manager) enqueueOrDigest(event notifications.NotificationEvent) {
	if m.shouldDigest(string(event.Type)) {
		if err := m.store.InsertDigestEvent(event.MonitorID, event.MonitorName, event.MonitorURL, string(event.Type), event.Message, event.Time); err != nil {
			log.Printf("Failed to queue digest event: %v", err)
		}
		return
	}
	m.notifier.Enqueue(event)
}

// GetMonitor returns a specific monitor instance
func (m *Manager) GetMonitor(id string) *Monitor {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.monitors[id]
}

// RemoveMonitor explicitly stops and removes a monitor.
// This is useful for immediate cleanup after deletion.
// CheckNow runs a monitor's check immediately and waits briefly for the result, so a
// caller can verify a fix instead of waiting out the interval. Reports whether a fresh
// result arrived within the timeout.
func (m *Manager) CheckNow(id string, wait time.Duration) bool {
	mon := m.GetMonitor(id)
	if mon == nil {
		return false
	}

	before := len(mon.GetHistory())
	mon.ScheduleNow()

	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if len(mon.GetHistory()) > before {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

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

func (m *Manager) digestWorker() {
	m.wg.Add(1)
	defer m.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	lastSentDate := ""

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.mu.RLock()
			enabled := m.digestEnabled
			digestTime := m.digestTime
			loc := m.notificationTimezone
			m.mu.RUnlock()

			if !enabled {
				continue
			}

			now := time.Now().In(loc)
			currentTime := now.Format("15:04")
			currentDate := now.Format("2006-01-02")

			// >= rather than == ensures a tick that lands slightly past the target
			// minute still triggers the digest rather than waiting a full day.
			// lastSentDate guarantees exactly one send per calendar day.
			if currentTime >= digestTime && lastSentDate != currentDate {
				// Fetch events before recording the send so that a transient store
				// error causes a retry on the next tick instead of silently skipping
				// the day.
				events, err := m.store.GetUnsentDigestEvents()
				if err != nil {
					log.Printf("Digest: failed to get events: %v", err)
					continue
				}

				// Always call SendDigest: it delivers an all-clear message when the
				// event list is empty so operators receive a daily confirmation even
				// on incident-free days.
				m.notifier.SendDigest(events)

				if len(events) > 0 {
					ids := make([]int64, len(events))
					for i, e := range events {
						ids[i] = e.ID
					}
					if err := m.store.MarkDigestEventsSent(ids); err != nil {
						log.Printf("Digest: failed to mark events as sent: %v", err)
					}
				}

				lastSentDate = currentDate
				log.Printf("Digest: sent daily summary with %d events for %s", len(events), currentDate)
			}
		}
	}
}

func (m *Manager) retentionWorker() {
	m.wg.Add(1)
	defer m.wg.Done()

	prune := func() {
		days := 365 // Default
		if val, err := m.store.GetSetting("data_retention_days"); err == nil {
			if i, err := strconv.Atoi(val); err == nil && i > 0 {
				days = i
			}
		}
		if err := m.store.PruneMonitorChecks(days); err != nil {
			log.Printf("Retention error: %v", err)
		}
		if err := m.store.PruneDigestEvents(days); err != nil {
			log.Printf("Retention: failed to prune digest events: %v", err)
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
