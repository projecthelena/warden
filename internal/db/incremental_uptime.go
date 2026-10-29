package db

import (
	"database/sql"
	"fmt"
	"time"
)

const incrementalUptimeMarker = "uptime.incremental.initialized"

func (s *Store) IncrementalUptimeEnabled() bool { return s.incrementalUptime }

// configureUptime runs before ingestion. Writers must use the same mode and stop
// before switching it; a disabled startup invalidates the next incremental baseline.
func (s *Store) configureUptime() error {
	days, _ := s.RetentionDays()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if !s.incrementalUptime {
		if _, err = tx.Exec(s.rebind("DELETE FROM settings WHERE key = ?"), incrementalUptimeMarker); err != nil {
			return err
		}
		return tx.Commit()
	}
	var initialized string
	err = tx.QueryRow(s.rebind("SELECT value FROM settings WHERE key = ?"), incrementalUptimeMarker).Scan(&initialized)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if initialized == "true" {
		return tx.Commit()
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02")
	day := "date(timestamp)"
	if s.IsPostgres() {
		day = "TO_CHAR(timestamp,'YYYY-MM-DD')"
	}
	query := `INSERT INTO monitor_uptime_daily (monitor_id,day,total,up_count)
 SELECT monitor_id,` + day + `,COUNT(*),SUM(CASE WHEN status='up' THEN 1 ELSE 0 END)
 FROM monitor_checks GROUP BY monitor_id,` + day + `
 ON CONFLICT (monitor_id,day) DO UPDATE SET total=excluded.total,up_count=excluded.up_count
 WHERE excluded.day > ?`
	if _, err = tx.Exec(s.rebind(query), cutoff); err != nil {
		return fmt.Errorf("initialize incremental uptime: %w", err)
	}
	if _, err = tx.Exec(s.rebind("INSERT INTO settings (key,value) VALUES (?, 'true') ON CONFLICT(key) DO UPDATE SET value=excluded.value"), incrementalUptimeMarker); err != nil {
		return err
	}
	return tx.Commit()
}
