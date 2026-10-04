package db

import "time"

type CheckSummary struct {
	Since         time.Time        `json:"since"`
	Until         time.Time        `json:"until"`
	Total         int64            `json:"total"`
	Failed        int64            `json:"failed"`
	Recovered     int64            `json:"recovered"`
	Traced        int64            `json:"traced"`
	FailurePhases map[string]int64 `json:"failurePhases"`
}

// GetMonitorCheckSummary counts checks, not attempts. A successful retry remains
// visible as a recovery, even though it did not become a down result.
func (s *Store) GetMonitorCheckSummary(id string, since, until time.Time) (CheckSummary, error) {
	summary := CheckSummary{Since: since, Until: until, FailurePhases: map[string]int64{}}
	phase := "COALESCE(json_extract(diagnostics, '$.attempts[0].failurePhase'), '')"
	attempts := "COALESCE(json_array_length(diagnostics, '$.attempts'), 0)"
	if s.IsPostgres() {
		phase = "COALESCE(diagnostics::jsonb->'attempts'->0->>'failurePhase', '')"
		attempts = "COALESCE(jsonb_array_length(diagnostics::jsonb->'attempts'), 0)"
	}
	query := `SELECT status, ` + phase + ` AS phase, CASE WHEN ` + attempts + ` > 1 THEN 1 ELSE 0 END AS retried, CASE WHEN diagnostics IS NOT NULL THEN 1 ELSE 0 END AS traced, COUNT(*) FROM monitor_checks WHERE monitor_id = ? AND timestamp >= ? AND timestamp < ? GROUP BY 1, 2, 3, 4`
	rows, err := s.db.Query(s.rebind(query), id, since, until)
	if err != nil {
		return summary, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var status, phase string
		var retried, traced int
		var count int64
		if err := rows.Scan(&status, &phase, &retried, &traced, &count); err != nil {
			return summary, err
		}
		summary.Total += count
		if status == "down" {
			summary.Failed += count
		}
		if status == "up" && retried == 1 {
			summary.Recovered += count
		}
		if traced == 1 {
			summary.Traced += count
		}
		if phase != "" {
			summary.FailurePhases[phase] += count
		}
	}
	return summary, rows.Err()
}
