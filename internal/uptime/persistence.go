package uptime

import (
	"log"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/metricstate"
	"github.com/projecthelena/warden/internal/observability"
)

func (m *Manager) persistBatch(batch []db.CheckResult) error {
	delay := 100 * time.Millisecond
	for {
		started := metricstate.Now()
		if metricstate.Enabled() {
			observability.PersistBatchSize.Observe(float64(len(batch)))
		}
		err := m.store.BatchInsertChecksContext(m.persistCtx, batch)
		if metricstate.Enabled() {
			outcome := "success"
			if err != nil {
				outcome = "error"
			}
			observability.PersistDuration.WithLabelValues(outcome).Observe(time.Since(started).Seconds())
		}
		if err == nil || !m.store.IncrementalUptimeEnabled() || !db.RetryableCheckError(err) {
			return err
		}
		log.Printf("Check batch persistence failed; retrying in %s", delay)
		timer := time.NewTimer(delay)
		select {
		case <-m.persistCtx.Done():
			timer.Stop()
			return m.persistCtx.Err()
		case <-timer.C:
		}
		delay = min(delay*2, 5*time.Second)
	}
}
