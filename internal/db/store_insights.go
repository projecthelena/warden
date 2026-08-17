package db

import (
	"encoding/json"
	"time"
)

// MonitorInsight is one stored finding from the pattern detectors.
type MonitorInsight struct {
	ID          int64          `json:"id"`
	MonitorID   string         `json:"monitorId"`
	MonitorName string         `json:"monitorName"`
	Kind        string         `json:"kind"`
	Summary     string         `json:"summary"`
	Detail      map[string]any `json:"detail,omitempty"`
	Confidence  string         `json:"confidence"`
	DetectedAt  time.Time      `json:"detectedAt"`
}

// ReplaceMonitorInsights swaps a monitor's findings for a freshly computed set. Replacing
// wholesale rather than appending is what makes a pattern that stopped happening stop
// being reported — a stale finding is worse than none, because it sends someone looking
// for something that is no longer there.
func (s *Store) ReplaceMonitorInsights(monitorID string, findings []MonitorInsight, now time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(s.rebind("DELETE FROM monitor_insights WHERE monitor_id = ?"), monitorID); err != nil {
		return err
	}

	for _, f := range findings {
		var detail any
		if len(f.Detail) > 0 {
			b, err := json.Marshal(f.Detail)
			if err != nil {
				return err
			}
			detail = string(b)
		}
		if _, err := tx.Exec(s.rebind(`
			INSERT INTO monitor_insights (monitor_id, kind, summary, detail, confidence, detected_at)
			VALUES (?, ?, ?, ?, ?, ?)`),
			monitorID, f.Kind, f.Summary, detail, f.Confidence, now.UTC()); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetMonitorInsights returns stored findings, optionally for a single monitor.
func (s *Store) GetMonitorInsights(monitorID string) ([]MonitorInsight, error) {
	query := `
		SELECT i.id, i.monitor_id, m.name, i.kind, i.summary, COALESCE(i.detail, ''), i.confidence, i.detected_at
		FROM monitor_insights i
		JOIN monitors m ON i.monitor_id = m.id`
	args := []any{}
	if monitorID != "" {
		query += " WHERE i.monitor_id = ?"
		args = append(args, monitorID)
	}
	query += " ORDER BY i.detected_at DESC, i.id ASC"

	rows, err := s.db.Query(s.rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []MonitorInsight
	for rows.Next() {
		var in MonitorInsight
		var detail string
		if err := rows.Scan(&in.ID, &in.MonitorID, &in.MonitorName, &in.Kind, &in.Summary,
			&detail, &in.Confidence, &in.DetectedAt); err != nil {
			return nil, err
		}
		if detail != "" {
			// A finding whose detail will not parse is still worth showing: the summary is
			// the part a human reads.
			_ = json.Unmarshal([]byte(detail), &in.Detail)
		}
		out = append(out, in)
	}
	return out, rows.Err()
}

// HourlyLatency returns one average-latency point per hour for a monitor. Hourly is the
// resolution the shapes live at: finer buries a four-hour ramp in per-check noise, coarser
// erases it entirely.
func (s *Store) HourlyLatency(monitorID string, since time.Time) ([]LatencyPoint, error) {
	group := "strftime('%Y-%m-%d %H:00:00', timestamp)"
	if s.IsPostgres() {
		group = "TO_CHAR(timestamp, 'YYYY-MM-DD HH24:00:00')"
	}

	rows, err := s.db.Query(s.rebind(`
		SELECT `+group+` AS ts_group,
		       CAST(AVG(latency) AS INTEGER) AS avg_latency,
		       MAX(CASE WHEN status != 'up' THEN 1 ELSE 0 END) AS failed
		FROM monitor_checks
		WHERE monitor_id = ? AND timestamp >= ?
		GROUP BY ts_group
		ORDER BY ts_group ASC`), monitorID, since.UTC())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []LatencyPoint
	for rows.Next() {
		var tsStr string
		var latency int64
		var failed int
		if err := rows.Scan(&tsStr, &latency, &failed); err != nil {
			return nil, err
		}
		ts, err := time.Parse("2006-01-02 15:04:05", tsStr)
		if err != nil {
			continue
		}
		out = append(out, LatencyPoint{Timestamp: ts.UTC(), Latency: latency, Failed: failed == 1})
	}
	return out, rows.Err()
}

// EventTimes returns when a monitor produced events of the given types. Used to ask whether
// its trouble clusters at a particular time of day.
func (s *Store) EventTimes(monitorID string, types []string, since time.Time) ([]time.Time, error) {
	if len(types) == 0 {
		return nil, nil
	}

	query := "SELECT timestamp FROM monitor_events WHERE monitor_id = ? AND timestamp >= ? AND type IN ("
	args := []any{monitorID, since.UTC()}
	for i, t := range types {
		if i > 0 {
			query += ", "
		}
		query += "?"
		args = append(args, t)
	}
	query += ") ORDER BY timestamp ASC"

	rows, err := s.db.Query(s.rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []time.Time
	for rows.Next() {
		var ts time.Time
		if err := rows.Scan(&ts); err != nil {
			return nil, err
		}
		out = append(out, ts.UTC())
	}
	return out, rows.Err()
}

// OutageWindow is one outage reduced to its span, for overlap analysis.
type OutageWindow struct {
	MonitorID string
	Start     time.Time
	End       *time.Time
}

// OutageWindowsSince returns every outage that started in the window, across all monitors,
// so pairs can be compared without a query per pair.
func (s *Store) OutageWindowsSince(since time.Time) ([]OutageWindow, error) {
	rows, err := s.db.Query(s.rebind(`
		SELECT monitor_id, start_time, end_time
		FROM monitor_outages
		WHERE start_time >= ?
		ORDER BY start_time ASC`), since.UTC())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []OutageWindow
	for rows.Next() {
		var w OutageWindow
		if err := rows.Scan(&w.MonitorID, &w.Start, &w.End); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// CountOutagesSince counts a monitor's outages in a window, the input to "this one is
// broken far more often than everything else".
func (s *Store) CountOutagesSince(monitorID string, since time.Time) (int, error) {
	var n int
	err := s.db.QueryRow(s.rebind(
		"SELECT COUNT(*) FROM monitor_outages WHERE monitor_id = ? AND start_time >= ?"),
		monitorID, since.UTC()).Scan(&n)
	return n, err
}
