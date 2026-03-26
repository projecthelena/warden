package db

import (
	"fmt"
	"log"
	"strings"
	"time"
)

// Settings

func (s *Store) GetSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow(s.rebind("SELECT value FROM settings WHERE key = ?"), key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

func (s *Store) SetSetting(key, value string) error {
	var err error
	if s.IsPostgres() {
		_, err = s.db.Exec("INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT(key) DO UPDATE SET value = excluded.value", key, value)
	} else {
		// SQLite: INSERT OR REPLACE
		_, err = s.db.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)", key, value)
	}
	return err
}

// Notification Channels

type NotificationChannel struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	Config    string    `json:"config"` // JSON string
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *Store) CreateNotificationChannel(c NotificationChannel) error {
	_, err := s.db.Exec(s.rebind("INSERT INTO notification_channels (id, type, name, config, enabled, created_at) VALUES (?, ?, ?, ?, ?, ?)"),
		c.ID, c.Type, c.Name, c.Config, c.Enabled, time.Now())
	return err
}

func (s *Store) GetNotificationChannels() ([]NotificationChannel, error) {
	rows, err := s.db.Query("SELECT id, type, name, config, enabled, created_at FROM notification_channels ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var channels []NotificationChannel
	for rows.Next() {
		var c NotificationChannel
		if err := rows.Scan(&c.ID, &c.Type, &c.Name, &c.Config, &c.Enabled, &c.CreatedAt); err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return channels, nil
}

func (s *Store) UpdateNotificationChannel(id, name, channelType, config string, enabled bool) error {
	_, err := s.db.Exec(s.rebind("UPDATE notification_channels SET name = ?, type = ?, config = ?, enabled = ? WHERE id = ?"),
		name, channelType, config, enabled, id)
	return err
}

func (s *Store) DeleteNotificationChannel(id string) error {
	_, err := s.db.Exec(s.rebind("DELETE FROM notification_channels WHERE id = ?"), id)
	return err
}

// System Stats

type SystemStats struct {
	TotalMonitors    int `json:"totalMonitors"`
	ActiveMonitors   int `json:"activeMonitors"`
	DownMonitors     int `json:"downMonitors"`
	DegradedMonitors int `json:"degradedMonitors"`
	TotalGroups      int `json:"totalGroups"`
	DailyPings       int `json:"dailyPingsEstimate"`
}

type SystemEvent struct {
	ID          int64     `json:"id"`
	MonitorID   string    `json:"monitorId"`
	MonitorName string    `json:"monitorName"`
	Type        string    `json:"type"` // up, down, degraded
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
}

// GetSystemEvents returns all events for all monitors
func (s *Store) GetSystemEvents(limit int) ([]SystemEvent, error) {
	query := `
		SELECT e.id, e.monitor_id, m.name, e.type, e.message, e.timestamp
		FROM monitor_events e
		JOIN monitors m ON e.monitor_id = m.id
		ORDER BY e.timestamp ASC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var events []SystemEvent
	for rows.Next() {
		var e SystemEvent
		if err := rows.Scan(&e.ID, &e.MonitorID, &e.MonitorName, &e.Type, &e.Message, &e.Timestamp); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func (s *Store) GetSystemStats() (*SystemStats, error) {
	stats := &SystemStats{}

	// Monitor Counts
	if err := s.db.QueryRow("SELECT COUNT(*) FROM monitors").Scan(&stats.TotalMonitors); err != nil {
		log.Printf("Failed to scan total monitors: %v", err)
	}
	var activeQuery string
	if s.IsPostgres() {
		activeQuery = "SELECT COUNT(*) FROM monitors WHERE active = TRUE"
	} else {
		activeQuery = "SELECT COUNT(*) FROM monitors WHERE active = 1"
	}
	if err := s.db.QueryRow(activeQuery).Scan(&stats.ActiveMonitors); err != nil {
		log.Printf("Failed to scan active monitors: %v", err)
	}

	if err := s.db.QueryRow("SELECT COUNT(DISTINCT monitor_id) FROM monitor_outages WHERE end_time IS NULL AND type = 'down'").Scan(&stats.DownMonitors); err != nil {
		log.Printf("Failed to scan down monitors: %v", err)
	}
	if err := s.db.QueryRow("SELECT COUNT(DISTINCT monitor_id) FROM monitor_outages WHERE end_time IS NULL AND type = 'degraded'").Scan(&stats.DegradedMonitors); err != nil {
		log.Printf("Failed to scan degraded monitors: %v", err)
	}

	// Groups
	if err := s.db.QueryRow("SELECT COUNT(*) FROM groups").Scan(&stats.TotalGroups); err != nil {
		log.Printf("Failed to scan groups: %v", err)
	}

	// Daily Pings Estimate
	var pingsQuery string
	if s.IsPostgres() {
		pingsQuery = "SELECT COALESCE(SUM(86400 / interval_seconds), 0) FROM monitors WHERE active = TRUE"
	} else {
		pingsQuery = "SELECT COALESCE(SUM(86400 / interval_seconds), 0) FROM monitors WHERE active = 1"
	}
	if err := s.db.QueryRow(pingsQuery).Scan(&stats.DailyPings); err != nil {
		log.Printf("Failed to scan daily pings: %v", err)
	}

	return stats, nil
}

// DigestEvent represents a queued notification for the daily digest.
type DigestEvent struct {
	ID          int64     `json:"id"`
	MonitorID   string    `json:"monitorId"`
	MonitorName string    `json:"monitorName"`
	MonitorURL  string    `json:"monitorUrl"`
	EventType   string    `json:"eventType"`
	Message     string    `json:"message"`
	EventTime   time.Time `json:"eventTime"`
}

// InsertDigestEvent queues a notification event for the daily digest.
func (s *Store) InsertDigestEvent(monitorID, monitorName, monitorURL, eventType, message string, eventTime time.Time) error {
	_, err := s.db.Exec(s.rebind("INSERT INTO notification_digest_queue (monitor_id, monitor_name, monitor_url, event_type, message, event_time) VALUES (?, ?, ?, ?, ?, ?)"),
		monitorID, monitorName, monitorURL, eventType, message, eventTime)
	return err
}

// GetUnsentDigestEvents retrieves all queued digest events that have not yet been sent.
// Events are NOT deleted; call MarkDigestEventsSent after a successful send.
func (s *Store) GetUnsentDigestEvents() ([]DigestEvent, error) {
	rows, err := s.db.Query(s.rebind("SELECT id, monitor_id, monitor_name, monitor_url, event_type, message, event_time FROM notification_digest_queue WHERE sent = ? ORDER BY event_time ASC"), false)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var events []DigestEvent
	for rows.Next() {
		var e DigestEvent
		if err := rows.Scan(&e.ID, &e.MonitorID, &e.MonitorName, &e.MonitorURL, &e.EventType, &e.Message, &e.EventTime); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

// MarkDigestEventsSent marks the given digest event IDs as sent so they are not
// re-dispatched on the next digest run. Old sent events are cleaned up by PruneDigestEvents.
func (s *Store) MarkDigestEventsSent(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	var query string
	if s.IsPostgres() {
		placeholders := make([]string, len(ids))
		for i := range ids {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}
		query = fmt.Sprintf("UPDATE notification_digest_queue SET sent = TRUE, sent_at = NOW() WHERE id IN (%s)",
			strings.Join(placeholders, ", "))
	} else {
		placeholders := make([]string, len(ids))
		for i := range ids {
			placeholders[i] = "?"
		}
		query = fmt.Sprintf("UPDATE notification_digest_queue SET sent = 1, sent_at = datetime('now') WHERE id IN (%s)",
			strings.Join(placeholders, ", "))
	}

	_, err := s.db.Exec(query, args...)
	return err
}

// PruneDigestEvents deletes sent digest events older than the given number of days.
// This is called by the retention worker alongside PruneMonitorChecks.
func (s *Store) PruneDigestEvents(days int) error {
	if days < 1 || days > 3650 {
		return fmt.Errorf("invalid retention days: must be between 1 and 3650")
	}

	var err error
	if s.IsPostgres() {
		_, err = s.db.Exec("DELETE FROM notification_digest_queue WHERE sent = TRUE AND sent_at < NOW() - MAKE_INTERVAL(days => $1)", days)
	} else {
		_, err = s.db.Exec("DELETE FROM notification_digest_queue WHERE sent = 1 AND sent_at < datetime('now', '-' || ? || ' days')", days)
	}
	return err
}

func (s *Store) GetDBSize() (int64, error) {
	if s.IsPostgres() {
		// PostgreSQL: use pg_database_size()
		var size int64
		if err := s.db.QueryRow("SELECT pg_database_size(current_database())").Scan(&size); err != nil {
			return 0, err
		}
		return size, nil
	}

	// SQLite: PRAGMA page_count * PRAGMA page_size
	var pageCount int64
	var pageSize int64
	if err := s.db.QueryRow("PRAGMA page_count").Scan(&pageCount); err != nil {
		return 0, err
	}
	if err := s.db.QueryRow("PRAGMA page_size").Scan(&pageSize); err != nil {
		return 0, err
	}
	return pageCount * pageSize, nil
}
