package uptime

import (
	"log/slog"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/observability"
)

const slowRollupThreshold = time.Second

func (m *Manager) runRollup(mode string, days int) error {
	if !m.RollupDiagnostics {
		return m.store.RollupDailyUptime(days)
	}
	start := time.Now()
	observability.SetRollupInProgress(mode, true)
	defer observability.SetRollupInProgress(mode, false)

	stats, err := m.store.RollupDailyUptimeWithStats(days)
	total := time.Since(start)
	observability.ObserveRollup(mode, stats, total, err)
	logSlowRollup(mode, m.store.Dialect(), stats, total, err)
	return err
}

func logSlowRollup(mode, dialect string, stats db.DailyUptimeRollupStats, total time.Duration, err error) {
	if total < slowRollupThreshold {
		return
	}
	outcome := "success"
	if err != nil {
		outcome = "error"
	}
	// No SQL, monitor identifiers, targets, connection strings or driver errors.
	slog.Warn("slow uptime rollup",
		"mode", mode, "dialect", dialect, "outcome", outcome, "rows", stats.Rows,
		"total", total, "read_connection", stats.ReadConnectionDuration,
		"query", stats.QueryDuration, "scan", stats.ScanDuration,
		"write_connection", stats.WriteConnectionDuration, "begin", stats.BeginDuration,
		"upsert", stats.UpsertDuration, "commit", stats.CommitDuration,
		slog.Group("pool_before", "open", stats.DBBefore.OpenConnections,
			"in_use", stats.DBBefore.InUse, "idle", stats.DBBefore.Idle,
			"wait_count", stats.DBBefore.WaitCount, "wait_duration", stats.DBBefore.WaitDuration),
		slog.Group("pool_after", "open", stats.DBAfter.OpenConnections,
			"in_use", stats.DBAfter.InUse, "idle", stats.DBAfter.Idle,
			"wait_count", stats.DBAfter.WaitCount, "wait_duration", stats.DBAfter.WaitDuration),
	)
}
