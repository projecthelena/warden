package db

import (
	"database/sql"
	"errors"
	"time"
)

var ErrGroupNotFound = errors.New("group not found")

type Group struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Monitors  []Monitor `json:"monitors"`
	CreatedAt time.Time `json:"createdAt"`
}

// Group CRUD

func (s *Store) CreateGroup(g Group) error {
	_, err := s.db.Exec(s.rebind("INSERT INTO groups (id, name, created_at) VALUES (?, ?, ?)"), g.ID, g.Name, time.Now())
	return err
}

func (s *Store) DeleteGroup(id string) error {
	_, err := s.db.Exec(s.rebind("DELETE FROM groups WHERE id = ?"), id)
	return err
}

func (s *Store) UpdateGroup(id, name string) error {
	_, err := s.db.Exec(s.rebind("UPDATE groups SET name = ? WHERE id = ?"), name, id)
	return err
}

// GetGroup returns group metadata without loading any of its monitors. Group detail
// endpoints use this instead of GetGroups so a request for one page does not first
// materialize the entire monitor collection.
func (s *Store) GetGroup(id string) (Group, error) {
	var g Group
	err := s.db.QueryRow(s.rebind("SELECT id, name, created_at FROM groups WHERE id = ?"), id).
		Scan(&g.ID, &g.Name, &g.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Group{}, ErrGroupNotFound
	}
	if err != nil {
		return Group{}, err
	}
	g.Monitors = []Monitor{}
	return g, nil
}

func (s *Store) GetGroups() ([]Group, error) {
	groups, err := s.GetGroupsMetadata()
	if err != nil {
		return nil, err
	}

	groupMap := make(map[string]*Group)
	for i := range groups {
		groupMap[groups[i].ID] = &groups[i]
	}

	// Fetch Monitors
	mRows, err := s.db.Query("SELECT id, group_id, name, url, active, interval_seconds, created_at FROM monitors ORDER BY created_at ASC")
	if err != nil {
		return nil, err
	}
	defer func() { _ = mRows.Close() }()

	for mRows.Next() {
		var m Monitor
		if err := mRows.Scan(&m.ID, &m.GroupID, &m.Name, &m.URL, &m.Active, &m.Interval, &m.CreatedAt); err != nil {
			return nil, err
		}
		if g, exists := groupMap[m.GroupID]; exists {
			g.Monitors = append(g.Monitors, m)
		}
	}

	return groups, mRows.Err()
}

// GetGroupsMetadata lists groups without loading their monitor collections. Use it for
// navigation and overview responses where embedding every monitor only wastes memory.
func (s *Store) GetGroupsMetadata() ([]Group, error) {
	var query string
	if s.IsPostgres() {
		query = "SELECT id, name, created_at FROM groups ORDER BY LOWER(name) ASC"
	} else {
		query = "SELECT id, name, created_at FROM groups ORDER BY name COLLATE NOCASE ASC"
	}
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	groups := []Group{}
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.CreatedAt); err != nil {
			return nil, err
		}
		g.Monitors = []Monitor{} // Initialize empty
		groups = append(groups, g)
	}
	return groups, rows.Err()
}
