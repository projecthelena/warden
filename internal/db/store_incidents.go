package db

import (
	"database/sql"
	"time"
)

type Incident struct {
	ID             string     `json:"id"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Type           string     `json:"type"`     // incident | maintenance
	Severity       string     `json:"severity"` // minor | major | critical
	Status         string     `json:"status"`   // investigation | identified | ... | scheduled | in_progress | completed
	StartTime      time.Time  `json:"startTime"`
	EndTime        *time.Time `json:"endTime,omitempty"`
	AffectedGroups string     `json:"affectedGroups"` // JSON array
	CreatedAt      time.Time  `json:"createdAt"`
}

func (s *Store) CreateIncident(i Incident) error {
	_, err := s.db.Exec(`
		INSERT INTO incidents (id, title, description, type, severity, status, start_time, end_time, affected_groups, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, i.ID, i.Title, i.Description, i.Type, i.Severity, i.Status, i.StartTime, i.EndTime, i.AffectedGroups, time.Now())
	return err
}

func (s *Store) GetIncidents(since time.Time) ([]Incident, error) {
	query := `
		SELECT id, title, description, type, severity, status, start_time, end_time, affected_groups, created_at 
		FROM incidents 
		WHERE (status != 'resolved' AND status != 'completed') 
		OR start_time >= ? 
		ORDER BY created_at DESC
	`
	rows, err := s.db.Query(query, since)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var incidents []Incident
	for rows.Next() {
		var i Incident
		var endTime sql.NullTime
		if err := rows.Scan(&i.ID, &i.Title, &i.Description, &i.Type, &i.Severity, &i.Status, &i.StartTime, &endTime, &i.AffectedGroups, &i.CreatedAt); err != nil {
			return nil, err
		}
		if endTime.Valid {
			i.EndTime = &endTime.Time
		}
		incidents = append(incidents, i)
	}
	return incidents, nil
}

func (s *Store) UpdateIncident(i Incident) error {
	_, err := s.db.Exec(`
		UPDATE incidents 
		SET title=?, description=?, type=?, severity=?, status=?, start_time=?, end_time=?, affected_groups=?
		WHERE id=?
	`, i.Title, i.Description, i.Type, i.Severity, i.Status, i.StartTime, i.EndTime, i.AffectedGroups, i.ID)
	return err
}

func (s *Store) DeleteIncident(id string) error {
	_, err := s.db.Exec("DELETE FROM incidents WHERE id = ?", id)
	return err
}
