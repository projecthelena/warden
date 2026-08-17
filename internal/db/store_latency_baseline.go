package db

import (
	"database/sql"
	"time"
)

// LatencyBaseline is what "normal" looks like for one monitor, from its own history. A
// fixed global threshold cannot serve a health check that answers in 254ms and a homepage
// that answers in 427ms; this is the per-monitor answer to the same question.
type LatencyBaseline struct {
	MonitorID  string
	P50        int64
	P95        int64
	Samples    int
	ComputedAt time.Time
}

// GetLatencyBaselines returns every stored baseline, keyed by monitor id.
func (s *Store) GetLatencyBaselines() (map[string]LatencyBaseline, error) {
	rows, err := s.db.Query("SELECT monitor_id, p50, p95, samples, computed_at FROM monitor_latency_baseline")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]LatencyBaseline)
	for rows.Next() {
		var b LatencyBaseline
		if err := rows.Scan(&b.MonitorID, &b.P50, &b.P95, &b.Samples, &b.ComputedAt); err != nil {
			return nil, err
		}
		out[b.MonitorID] = b
	}
	return out, rows.Err()
}

// percentileLatency returns the value at the given percentile of a monitor's successful
// checks since `since`, using OFFSET rather than a percentile function so the same query
// runs on both SQLite and Postgres.
//
// Only successful checks count. A failed check's latency is the time spent failing, which
// says nothing about how fast the service is when it works.
func (s *Store) percentileLatency(monitorID string, since time.Time, total, percentile int) (int64, error) {
	offset := total * percentile / 100
	if offset >= total {
		offset = total - 1
	}
	if offset < 0 {
		offset = 0
	}

	var v int64
	err := s.db.QueryRow(s.rebind(`
		SELECT latency FROM monitor_checks
		WHERE monitor_id = ? AND status = 'up' AND timestamp >= ?
		ORDER BY latency ASC
		LIMIT 1 OFFSET ?`), monitorID, since.UTC(), offset).Scan(&v)
	return v, err
}

// ComputeLatencyBaseline recalculates one monitor's baseline over the given window and
// stores it. Monitors with fewer than minSamples successful checks are left alone: an
// unreliable baseline is worse than no baseline, because everything downstream trusts it.
// Reports whether a baseline was written.
func (s *Store) ComputeLatencyBaseline(monitorID string, window time.Duration, minSamples int, now time.Time) (bool, error) {
	since := now.Add(-window)

	var total int
	err := s.db.QueryRow(s.rebind(
		"SELECT COUNT(*) FROM monitor_checks WHERE monitor_id = ? AND status = 'up' AND timestamp >= ?"),
		monitorID, since.UTC()).Scan(&total)
	if err != nil {
		return false, err
	}
	if total < minSamples {
		return false, nil
	}

	p50, err := s.percentileLatency(monitorID, since, total, 50)
	if err != nil {
		return false, err
	}
	p95, err := s.percentileLatency(monitorID, since, total, 95)
	if err != nil {
		return false, err
	}

	return true, s.upsertLatencyBaseline(LatencyBaseline{
		MonitorID: monitorID, P50: p50, P95: p95, Samples: total, ComputedAt: now.UTC(),
	})
}

func (s *Store) upsertLatencyBaseline(b LatencyBaseline) error {
	query := `
		INSERT INTO monitor_latency_baseline (monitor_id, p50, p95, samples, computed_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (monitor_id) DO UPDATE SET
			p50 = EXCLUDED.p50, p95 = EXCLUDED.p95,
			samples = EXCLUDED.samples, computed_at = EXCLUDED.computed_at`
	_, err := s.db.Exec(s.rebind(query), b.MonitorID, b.P50, b.P95, b.Samples, b.ComputedAt.UTC())
	return err
}

// GetLatencyBaseline returns one monitor's baseline, or false when it has none yet.
func (s *Store) GetLatencyBaseline(monitorID string) (LatencyBaseline, bool, error) {
	var b LatencyBaseline
	err := s.db.QueryRow(s.rebind(
		"SELECT monitor_id, p50, p95, samples, computed_at FROM monitor_latency_baseline WHERE monitor_id = ?"),
		monitorID).Scan(&b.MonitorID, &b.P50, &b.P95, &b.Samples, &b.ComputedAt)
	if err == sql.ErrNoRows {
		return b, false, nil
	}
	if err != nil {
		return b, false, err
	}
	return b, true, nil
}
