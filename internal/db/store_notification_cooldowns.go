package db

import "time"

// ClaimNotificationCooldown persists the limit across process restarts. The
// conditional upsert also prevents concurrent callers from claiming the same slot.
func (s *Store) ClaimNotificationCooldown(monitorID, eventType string, now time.Time, cooldown time.Duration) (bool, error) {
	result, err := s.db.Exec(s.rebind(`
  INSERT INTO notification_cooldowns (monitor_id, event_type, notified_at)
  VALUES (?, ?, ?)
  ON CONFLICT (monitor_id, event_type) DO UPDATE SET notified_at = excluded.notified_at
  WHERE notification_cooldowns.notified_at <= ?`), monitorID, eventType, now.UTC(), now.Add(-cooldown).UTC())
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}
