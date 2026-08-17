package db

import (
	"database/sql"
	"time"
)

// OpenOutage is an outage that has not ended yet, carried together with everything the
// alerting layer needs to decide whether to speak about it and what to say. NotifiedAt is
// nil while the outage is still inside its silent window; once it is set the outage has
// been announced and its recovery is worth announcing too.
type OpenOutage struct {
	ID             int64
	MonitorID      string
	MonitorName    string
	MonitorURL     string
	GroupID        string
	GroupName      string
	Type           string // "down" or "degraded"
	Summary        string
	StartTime      time.Time
	NotifiedAt     *time.Time
	LastReminderAt *time.Time
	CorrelationID  string
	AlertsMuted    bool
}

// GetOpenOutages returns every outage without an end time, oldest first. The alerting
// evaluator polls this on every tick, so it stays a single indexed query. Oldest-first
// matters for correlation: the earliest member of a cluster anchors its time window.
func (s *Store) GetOpenOutages() ([]OpenOutage, error) {
	rows, err := s.db.Query(s.rebind(`
		SELECT o.id, o.monitor_id, m.name, m.url, m.group_id, g.name, o.type,
		       COALESCE(o.summary, ''), o.start_time, o.notified_at, o.last_reminder_at,
		       COALESCE(o.correlation_id, ''), m.alerts_muted
		FROM monitor_outages o
		JOIN monitors m ON o.monitor_id = m.id
		JOIN groups g ON m.group_id = g.id
		WHERE o.end_time IS NULL
		ORDER BY o.start_time ASC`))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []OpenOutage
	for rows.Next() {
		var o OpenOutage
		var notified, reminded sql.NullTime
		if err := rows.Scan(&o.ID, &o.MonitorID, &o.MonitorName, &o.MonitorURL, &o.GroupID,
			&o.GroupName, &o.Type, &o.Summary, &o.StartTime, &notified, &reminded,
			&o.CorrelationID, &o.AlertsMuted); err != nil {
			return nil, err
		}
		if notified.Valid {
			t := notified.Time
			o.NotifiedAt = &t
		}
		if reminded.Valid {
			t := reminded.Time
			o.LastReminderAt = &t
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// MarkOutageNotified stamps the moment an outage was first announced, and reports whether
// this call is the one that claimed it.
//
// Both conditions in the WHERE clause matter. `notified_at IS NULL` keeps the stamp
// idempotent, so two evaluator ticks racing on the same outage produce one alert.
// `end_time IS NULL` keeps it honest: the evaluator works from a snapshot, and the monitor
// can recover between reading that snapshot and acting on it. Without this guard the
// evaluator would stamp a row the result processor had already closed and announce a
// monitor that is back up — and because the recovery path checked `notified_at` before the
// stamp landed, no "recovered" would ever follow it.
func (s *Store) MarkOutageNotified(id int64, at time.Time) (bool, error) {
	res, err := s.db.Exec(s.rebind(
		"UPDATE monitor_outages SET notified_at = ? WHERE id = ? AND notified_at IS NULL AND end_time IS NULL"),
		at.UTC(), id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// MarkOutageReminded records that a reminder went out for an outage that is still open.
// Same race as MarkOutageNotified: an outage that closed since the snapshot was read has
// nothing left to remind anyone about.
func (s *Store) MarkOutageReminded(id int64, at time.Time) error {
	_, err := s.db.Exec(s.rebind(
		"UPDATE monitor_outages SET last_reminder_at = ? WHERE id = ? AND end_time IS NULL"), at.UTC(), id)
	return err
}

// CloseOutageReport closes any open outage for the monitor and reports whether the one it
// closed had already been announced. That answer is what decides if the recovery is worth
// a message: announcing "recovered" for something nobody was told about is pure noise.
//
// The read and the write are separate statements. The only writers for a given monitor are
// its own result-processing path and the startup reconciler, which never run concurrently
// for the same monitor, so a transaction would buy nothing here.
func (s *Store) CloseOutageReport(monitorID string) (bool, error) {
	var notified sql.NullTime
	err := s.db.QueryRow(s.rebind(
		"SELECT notified_at FROM monitor_outages WHERE monitor_id = ? AND end_time IS NULL ORDER BY start_time DESC LIMIT 1"),
		monitorID).Scan(&notified)
	if err != nil && err != sql.ErrNoRows {
		return false, err
	}
	wasNotified := err == nil && notified.Valid

	if err := s.CloseOutage(monitorID); err != nil {
		return false, err
	}
	return wasNotified, nil
}

// MarkOutagesNotified stamps a whole set of outages as announced under one correlation id.
// Used when several monitors fail together and get a single message: every member has to
// be stamped, or the ones left behind each fire their own alert a tick later.
//
// The notified_at IS NULL guard is per row, so a member that had already been announced on
// its own keeps its original stamp and is simply not double-counted.
func (s *Store) MarkOutagesNotified(ids []int64, at time.Time, correlationID string) (int, error) {
	claimed := 0
	for _, id := range ids {
		res, err := s.db.Exec(s.rebind(
			"UPDATE monitor_outages SET notified_at = ?, correlation_id = ? WHERE id = ? AND notified_at IS NULL"),
			at.UTC(), correlationID, id)
		if err != nil {
			return claimed, err
		}
		if n, err := res.RowsAffected(); err == nil && n > 0 {
			claimed++
		}
	}
	return claimed, nil
}

// CountAnnouncedOutagesSince counts how many times this monitor has interrupted someone in
// a window. It is the input to the repeat-offender damping: a monitor that has already
// alerted three times today is telling you about itself, not about an event.
func (s *Store) CountAnnouncedOutagesSince(monitorID string, since time.Time) (int, error) {
	var n int
	err := s.db.QueryRow(s.rebind(
		"SELECT COUNT(*) FROM monitor_outages WHERE monitor_id = ? AND notified_at IS NOT NULL AND notified_at >= ?"),
		monitorID, since.UTC()).Scan(&n)
	return n, err
}

// SetMonitorAlertsMuted flips the per-monitor mute. A muted monitor still records outages
// and still shows up in the daily digest; it just never interrupts anyone.
func (s *Store) SetMonitorAlertsMuted(monitorID string, muted bool) error {
	_, err := s.db.Exec(s.rebind("UPDATE monitors SET alerts_muted = ? WHERE id = ?"), muted, monitorID)
	return err
}
