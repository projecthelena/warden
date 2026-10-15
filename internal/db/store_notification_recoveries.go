package db

import (
	"database/sql"
	"time"
)

type PendingRecovery struct {
	MonitorID     string
	RecoveredAt   time.Time
	Payload       string
	CorrelationID string
}

// Closing the measured outage and queuing its notice are atomic. Delaying the
// notification must not inflate downtime or lose the recovery on restart.
func (s *Store) CloseOutageQueueRecovery(monitorID, payload string, recoveredAt time.Time, allowed bool) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var notified sql.NullTime
	var kind, correlation string
	err = tx.QueryRow(s.rebind(`SELECT notified_at, type, COALESCE(correlation_id, '') FROM monitor_outages WHERE monitor_id = ? AND end_time IS NULL ORDER BY start_time DESC LIMIT 1`), monitorID).Scan(&notified, &kind, &correlation)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if _, err = tx.Exec(s.rebind(`UPDATE monitor_outages SET end_time = ? WHERE monitor_id = ? AND end_time IS NULL`), recoveredAt.UTC(), monitorID); err != nil {
		return err
	}
	if notified.Valid && allowed {
		_, err = tx.Exec(s.rebind(`INSERT INTO notification_recoveries (monitor_id, recovered_at, notified_at, outage_type, correlation_id, payload)
   VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT (monitor_id) DO UPDATE SET recovered_at = excluded.recovered_at, notified_at = excluded.notified_at, outage_type = excluded.outage_type, correlation_id = excluded.correlation_id, payload = excluded.payload`), monitorID, recoveredAt.UTC(), notified.Time.UTC(), kind, correlation, payload)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) PendingNotificationRecoveries() ([]PendingRecovery, error) {
	rows, err := s.db.Query(`SELECT monitor_id, recovered_at, payload, correlation_id FROM notification_recoveries ORDER BY recovered_at, monitor_id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var result []PendingRecovery
	for rows.Next() {
		var r PendingRecovery
		if err := rows.Scan(&r.MonitorID, &r.RecoveredAt, &r.Payload, &r.CorrelationID); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) ClaimNotificationRecovery(monitorID string, recoveredAt time.Time) (bool, error) {
	result, err := s.db.Exec(s.rebind(`DELETE FROM notification_recoveries WHERE monitor_id = ? AND recovered_at = ? AND NOT EXISTS (SELECT 1 FROM monitor_outages WHERE monitor_outages.monitor_id = notification_recoveries.monitor_id AND end_time IS NULL)`), monitorID, recoveredAt.UTC())
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n > 0, err
}

func (s *Store) HasOpenNotificationGroup(correlationID string) (bool, error) {
	var count int
	err := s.db.QueryRow(s.rebind(`SELECT COUNT(*) FROM monitor_outages o JOIN monitors m ON m.id = o.monitor_id WHERE o.correlation_id = ? AND o.end_time IS NULL AND m.active = ? AND m.alerts_muted = ?`), correlationID, true, false).Scan(&count)
	return count > 0, err
}
