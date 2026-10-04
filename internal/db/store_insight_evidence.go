package db

import (
	"database/sql"
	"errors"
	"time"
)

type InsightCheck struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	HasTrace  bool      `json:"hasTrace"`
}

// FindInsightCheck selects an example within the detector's actual evidence window.
// A negative latency selects the first check; otherwise it selects the closest
// latency to the hourly measurement. Missing retained checks are not substituted.
func (s *Store) FindInsightCheck(monitorID string, since, until time.Time, status string, latency int64) (*InsightCheck, error) {
	query := `SELECT id, timestamp, CASE WHEN diagnostics IS NOT NULL THEN 1 ELSE 0 END FROM monitor_checks WHERE monitor_id = ? AND timestamp >= ? AND timestamp < ? AND status = ?`
	args := []any{monitorID, since.UTC(), until.UTC(), status}
	if latency >= 0 {
		query += " ORDER BY ABS(latency - ?), timestamp DESC, id DESC"
		args = append(args, latency)
	} else {
		query += " ORDER BY timestamp ASC, id ASC"
	}
	query += " LIMIT 1"
	var ref InsightCheck
	var traced int
	err := s.db.QueryRow(s.rebind(query), args...).Scan(&ref.ID, &ref.Timestamp, &traced)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ref.HasTrace = traced == 1
	return &ref, nil
}
