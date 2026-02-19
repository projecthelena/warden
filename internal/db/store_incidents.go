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
	Source         string     `json:"source"`            // "auto" | "manual"
	OutageID       *int64     `json:"outageId"`          // nullable FK to monitor_outages
	Public         bool       `json:"public"`            // visible on public status page
}

type IncidentUpdate struct {
	ID         int64     `json:"id"`
	IncidentID string    `json:"incidentId"`
	Status     string    `json:"status"`
	Message    string    `json:"message"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (s *Store) CreateIncident(i Incident) error {
	// Default source to "manual" if not set
	source := i.Source
	if source == "" {
		source = "manual"
	}

	_, err := s.db.Exec(s.rebind(`
		INSERT INTO incidents (id, title, description, type, severity, status, start_time, end_time, affected_groups, created_at, source, outage_id, public)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`), i.ID, i.Title, i.Description, i.Type, i.Severity, i.Status, i.StartTime, i.EndTime, i.AffectedGroups, time.Now(), source, i.OutageID, i.Public)
	return err
}

func (s *Store) GetIncidents(since time.Time) ([]Incident, error) {
	query := s.rebind(`
		SELECT id, title, description, type, severity, status, start_time, end_time, affected_groups, created_at,
		       COALESCE(source, 'manual') as source, outage_id, COALESCE(public, FALSE) as public
		FROM incidents
		WHERE (status != 'resolved' AND status != 'completed')
		OR start_time >= ?
		ORDER BY created_at DESC
	`)
	rows, err := s.db.Query(query, since)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var incidents []Incident
	for rows.Next() {
		var i Incident
		var endTime sql.NullTime
		var outageID sql.NullInt64
		if err := rows.Scan(&i.ID, &i.Title, &i.Description, &i.Type, &i.Severity, &i.Status, &i.StartTime, &endTime, &i.AffectedGroups, &i.CreatedAt, &i.Source, &outageID, &i.Public); err != nil {
			return nil, err
		}
		if endTime.Valid {
			i.EndTime = &endTime.Time
		}
		if outageID.Valid {
			i.OutageID = &outageID.Int64
		}
		incidents = append(incidents, i)
	}
	return incidents, nil
}

func (s *Store) GetIncidentByID(id string) (*Incident, error) {
	query := s.rebind(`
		SELECT id, title, description, type, severity, status, start_time, end_time, affected_groups, created_at,
		       COALESCE(source, 'manual') as source, outage_id, COALESCE(public, FALSE) as public
		FROM incidents
		WHERE id = ?
	`)
	var i Incident
	var endTime sql.NullTime
	var outageID sql.NullInt64
	err := s.db.QueryRow(query, id).Scan(&i.ID, &i.Title, &i.Description, &i.Type, &i.Severity, &i.Status, &i.StartTime, &endTime, &i.AffectedGroups, &i.CreatedAt, &i.Source, &outageID, &i.Public)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if endTime.Valid {
		i.EndTime = &endTime.Time
	}
	if outageID.Valid {
		i.OutageID = &outageID.Int64
	}
	return &i, nil
}

func (s *Store) UpdateIncident(i Incident) error {
	_, err := s.db.Exec(s.rebind(`
		UPDATE incidents
		SET title=?, description=?, type=?, severity=?, status=?, start_time=?, end_time=?, affected_groups=?, source=?, outage_id=?, public=?
		WHERE id=?
	`), i.Title, i.Description, i.Type, i.Severity, i.Status, i.StartTime, i.EndTime, i.AffectedGroups, i.Source, i.OutageID, i.Public, i.ID)
	return err
}

func (s *Store) SetIncidentPublic(id string, public bool) error {
	_, err := s.db.Exec(s.rebind(`UPDATE incidents SET public = ? WHERE id = ?`), public, id)
	return err
}

func (s *Store) DeleteIncident(id string) error {
	_, err := s.db.Exec(s.rebind("DELETE FROM incidents WHERE id = ?"), id)
	return err
}

// CreateIncidentUpdate adds a timeline entry to an incident
func (s *Store) CreateIncidentUpdate(incidentID, status, message string) error {
	_, err := s.db.Exec(s.rebind(`
		INSERT INTO incident_updates (incident_id, status, message, created_at)
		VALUES (?, ?, ?, ?)
	`), incidentID, status, message, time.Now())
	return err
}

// GetIncidentUpdates returns all updates for an incident in chronological order
func (s *Store) GetIncidentUpdates(incidentID string) ([]IncidentUpdate, error) {
	query := s.rebind(`
		SELECT id, incident_id, status, message, created_at
		FROM incident_updates
		WHERE incident_id = ?
		ORDER BY created_at ASC
	`)
	rows, err := s.db.Query(query, incidentID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var updates []IncidentUpdate
	for rows.Next() {
		var u IncidentUpdate
		if err := rows.Scan(&u.ID, &u.IncidentID, &u.Status, &u.Message, &u.CreatedAt); err != nil {
			return nil, err
		}
		updates = append(updates, u)
	}
	return updates, nil
}

// GetPublicResolvedIncidents returns resolved/completed incidents marked as public since the given time.
// Only returns actual incidents (type='incident'), not maintenance windows.
func (s *Store) GetPublicResolvedIncidents(since time.Time) ([]Incident, error) {
	query := s.rebind(`
		SELECT id, title, description, type, severity, status, start_time, end_time, affected_groups, created_at,
		       COALESCE(source, 'manual') as source, outage_id, COALESCE(public, FALSE) as public
		FROM incidents
		WHERE public = TRUE
		AND type = 'incident'
		AND (status = 'resolved' OR status = 'completed')
		AND start_time >= ?
		ORDER BY start_time DESC
	`)
	rows, err := s.db.Query(query, since)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var incidents []Incident
	for rows.Next() {
		var i Incident
		var endTime sql.NullTime
		var outageID sql.NullInt64
		if err := rows.Scan(&i.ID, &i.Title, &i.Description, &i.Type, &i.Severity, &i.Status, &i.StartTime, &endTime, &i.AffectedGroups, &i.CreatedAt, &i.Source, &outageID, &i.Public); err != nil {
			return nil, err
		}
		if endTime.Valid {
			i.EndTime = &endTime.Time
		}
		if outageID.Valid {
			i.OutageID = &outageID.Int64
		}
		incidents = append(incidents, i)
	}
	return incidents, nil
}
