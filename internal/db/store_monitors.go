package db

import (
	"database/sql"
	"fmt"
	"time"
)

type Monitor struct {
	ID        string    `json:"id"`
	GroupID   string    `json:"groupId"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Active    bool      `json:"active"`
	Interval  int       `json:"interval"` // Seconds
	CreatedAt time.Time `json:"createdAt"`
}

type CheckResult struct {
	MonitorID  string    `json:"monitorId"`
	Status     string    `json:"status"`
	Latency    int64     `json:"latency"`
	Timestamp  time.Time `json:"timestamp"`
	StatusCode int       `json:"statusCode"`
}

type MonitorEvent struct {
	ID        int       `json:"id"`
	MonitorID string    `json:"monitorId"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type MonitorOutage struct {
	ID          int64      `json:"id"`
	MonitorID   string     `json:"monitorId"`
	Type        string     `json:"type"`
	Summary     string     `json:"summary"`
	StartTime   time.Time  `json:"startTime"`
	EndTime     *time.Time `json:"endTime"`
	MonitorName string     `json:"monitorName"` // Joined
	GroupName   string     `json:"groupName"`   // Joined
	GroupID     string     `json:"groupId"`     // Joined
}

type LatencyPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Latency   int64     `json:"latency"`
	Failed    bool      `json:"failed"`
}

// Monitor CRUD

func (s *Store) CreateMonitor(m Monitor) error {
	if m.Interval < 1 {
		m.Interval = 60 // Default safety
	}
	_, err := s.db.Exec("INSERT INTO monitors (id, group_id, name, url, active, interval_seconds, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		m.ID, m.GroupID, m.Name, m.URL, m.Active, m.Interval, time.Now())
	return err
}

func (s *Store) UpdateMonitor(id, name, url string, interval int) error {
	if interval < 1 {
		interval = 60
	}
	res, err := s.db.Exec("UPDATE monitors SET name = ?, url = ?, interval_seconds = ?, active = ? WHERE id = ?", name, url, interval, true, id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("monitor not found")
	}
	return nil
}

func (s *Store) DeleteMonitor(id string) error {
	_, err := s.db.Exec("DELETE FROM monitors WHERE id = ?", id)
	return err
}

// GetMonitors returns all monitors
func (s *Store) GetMonitors() ([]Monitor, error) {
	rows, err := s.db.Query("SELECT id, group_id, name, url, active, interval_seconds, created_at FROM monitors ORDER BY created_at ASC")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var monitors []Monitor
	for rows.Next() {
		var m Monitor
		if err := rows.Scan(&m.ID, &m.GroupID, &m.Name, &m.URL, &m.Active, &m.Interval, &m.CreatedAt); err != nil {
			return nil, err
		}
		monitors = append(monitors, m)
	}
	return monitors, nil
}

// Events & Checks

func (s *Store) CreateEvent(monitorID, eventType, message string) error {
	_, err := s.db.Exec("INSERT INTO monitor_events (monitor_id, type, message) VALUES (?, ?, ?)",
		monitorID, eventType, message)
	return err
}

func (s *Store) CreateOutage(monitorID, eventType, summary string) error {
	_, err := s.db.Exec("INSERT INTO monitor_outages (monitor_id, type, summary) VALUES (?, ?, ?)",
		monitorID, eventType, summary)
	return err
}

func (s *Store) CloseOutage(monitorID string) error {
	// Close any active outages for this monitor
	_, err := s.db.Exec("UPDATE monitor_outages SET end_time = CURRENT_TIMESTAMP WHERE monitor_id = ? AND end_time IS NULL", monitorID)
	return err
}

func (s *Store) GetActiveOutages() ([]MonitorOutage, error) {
	query := `
		SELECT o.id, o.monitor_id, o.type, o.summary, o.start_time, m.name, g.name, g.id
		FROM monitor_outages o
		JOIN monitors m ON o.monitor_id = m.id
		JOIN groups g ON m.group_id = g.id
		WHERE o.end_time IS NULL
		ORDER BY o.start_time DESC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var outages []MonitorOutage
	for rows.Next() {
		var o MonitorOutage
		if err := rows.Scan(&o.ID, &o.MonitorID, &o.Type, &o.Summary, &o.StartTime, &o.MonitorName, &o.GroupName, &o.GroupID); err != nil {
			return nil, err
		}
		outages = append(outages, o)
	}
	return outages, nil
}

func (s *Store) GetResolvedOutages(since time.Time) ([]MonitorOutage, error) {
	query := `
		SELECT o.id, o.monitor_id, o.type, o.summary, o.start_time, o.end_time, m.name, g.name, g.id
		FROM monitor_outages o
		JOIN monitors m ON o.monitor_id = m.id
		JOIN groups g ON m.group_id = g.id
		WHERE o.end_time IS NOT NULL AND o.end_time >= ?
		ORDER BY o.end_time DESC
	`
	rows, err := s.db.Query(query, since)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var outages []MonitorOutage
	for rows.Next() {
		var o MonitorOutage
		var endTime sql.NullTime
		if err := rows.Scan(&o.ID, &o.MonitorID, &o.Type, &o.Summary, &o.StartTime, &endTime, &o.MonitorName, &o.GroupName, &o.GroupID); err != nil {
			return nil, err
		}
		if endTime.Valid {
			o.EndTime = &endTime.Time
		}
		outages = append(outages, o)
	}
	return outages, nil
}

func (s *Store) BatchInsertChecks(checks []CheckResult) error {
	if len(checks) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare("INSERT INTO monitor_checks (monitor_id, status, latency, timestamp, status_code) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	for _, c := range checks {
		_, err := stmt.Exec(c.MonitorID, c.Status, c.Latency, c.Timestamp, c.StatusCode)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetMonitorChecks returns the last N checks for a monitor
func (s *Store) GetMonitorChecks(monitorID string, limit int) ([]CheckResult, error) {
	query := `SELECT monitor_id, status, latency, timestamp, COALESCE(status_code, 0) FROM monitor_checks 
			  WHERE monitor_id = ? ORDER BY timestamp DESC LIMIT ?`

	rows, err := s.db.Query(query, monitorID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var checks []CheckResult
	for rows.Next() {
		var c CheckResult
		if err := rows.Scan(&c.MonitorID, &c.Status, &c.Latency, &c.Timestamp, &c.StatusCode); err != nil {
			return nil, err
		}
		checks = append(checks, c)
	}
	return checks, nil
}

func (s *Store) PruneMonitorChecks(days int) error {
	_, err := s.db.Exec("DELETE FROM monitor_checks WHERE timestamp < datetime('now', '-' || ? || ' days')", days)
	return err
}

func (s *Store) GetUptimeStats(monitorID string) (float64, float64, float64, error) {
	query := `
		SELECT 
			COUNT(CASE WHEN timestamp > datetime('now', '-1 days') THEN 1 END) as total_24h,
			COUNT(CASE WHEN timestamp > datetime('now', '-1 days') AND status = 'up' THEN 1 END) as up_24h,
			COUNT(CASE WHEN timestamp > datetime('now', '-7 days') THEN 1 END) as total_7d,
			COUNT(CASE WHEN timestamp > datetime('now', '-7 days') AND status = 'up' THEN 1 END) as up_7d,
			COUNT(CASE WHEN timestamp > datetime('now', '-30 days') THEN 1 END) as total_30d,
			COUNT(CASE WHEN timestamp > datetime('now', '-30 days') AND status = 'up' THEN 1 END) as up_30d
		FROM monitor_checks
		WHERE monitor_id = ?
	`
	var t24, u24, t7, u7, t30, u30 int
	err := s.db.QueryRow(query, monitorID).Scan(&t24, &u24, &t7, &u7, &t30, &u30)
	if err != nil {
		return 0, 0, 0, err
	}

	calc := func(up, total int) float64 {
		if total == 0 {
			return 100.0 // Assume 100% if no data? Or 0? Usually 100 or null. Let's return 100 for "No Downtime Recorded" vibe.
		}
		return (float64(up) / float64(total)) * 100.0

	}

	return calc(u24, t24), calc(u7, t7), calc(u30, t30), nil
}

func (s *Store) GetMonitorEvents(monitorID string, limit int) ([]MonitorEvent, error) {
	query := `SELECT id, monitor_id, type, message, timestamp FROM monitor_events 
	          WHERE monitor_id = ? ORDER BY timestamp DESC LIMIT ?`

	rows, err := s.db.Query(query, monitorID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var events []MonitorEvent
	for rows.Next() {
		var e MonitorEvent
		if err := rows.Scan(&e.ID, &e.MonitorID, &e.Type, &e.Message, &e.Timestamp); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func (s *Store) GetLatencyStats(monitorID string, hours int) ([]LatencyPoint, error) {
	var query string
	var groupBy string

	if hours <= 1 {
		// 1h -> Group by Minute
		groupBy = "strftime('%Y-%m-%d %H:%M:00', timestamp)"
	} else if hours <= 168 {
		// 24h & 7d -> Group by Hour
		groupBy = "strftime('%Y-%m-%d %H:00:00', timestamp)"
	} else {
		// 30d+ -> Group by Day
		groupBy = "strftime('%Y-%m-%d', timestamp)"
	}

	query = fmt.Sprintf(`
		SELECT 
			%s as ts_group,
			CAST(AVG(latency) AS INTEGER) as avg_latency,
			MAX(CASE WHEN status != 'up' THEN 1 ELSE 0 END) as failed
		FROM monitor_checks 
		WHERE monitor_id = ? 
		AND datetime(timestamp) > datetime('now', '-' || ? || ' hours')
		GROUP BY ts_group
		ORDER BY ts_group ASC
	`, groupBy)

	rows, err := s.db.Query(query, monitorID, hours)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var points []LatencyPoint
	for rows.Next() {
		var p LatencyPoint
		var tsStr string
		if err := rows.Scan(&tsStr, &p.Latency, &p.Failed); err != nil {
			return nil, err
		}

		// Parse string depending on group?
		// SQLite returns strict formats we requested.
		// If groupBy returns like "2025-12-10 03:00:00"
		if len(tsStr) == 10 { // YYYY-MM-DD
			p.Timestamp, _ = time.Parse("2006-01-02", tsStr)
		} else {
			p.Timestamp, _ = time.Parse("2006-01-02 15:04:05", tsStr)
		}
		points = append(points, p)
	}
	return points, nil
}
