package db

import (
	"log"
	"time"
)

// Settings

func (s *Store) GetSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec("INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", key, value)
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
	_, err := s.db.Exec("INSERT INTO notification_channels (id, type, name, config, enabled, created_at) VALUES (?, ?, ?, ?, ?, ?)",
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

func (s *Store) DeleteNotificationChannel(id string) error {
	_, err := s.db.Exec("DELETE FROM notification_channels WHERE id = ?", id)
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
	if err := s.db.QueryRow("SELECT COUNT(*) FROM monitors WHERE active = 1").Scan(&stats.ActiveMonitors); err != nil {
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
	if err := s.db.QueryRow("SELECT COALESCE(SUM(86400 / interval_seconds), 0) FROM monitors WHERE active = 1").Scan(&stats.DailyPings); err != nil {
		log.Printf("Failed to scan daily pings: %v", err)
	}

	return stats, nil
}

func (s *Store) GetDBSize() (int64, error) {
	// PRAGMA page_count * PRAGMA page_size
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
