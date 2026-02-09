package db

import (
	"database/sql"
	"time"
)

// StatusPage Struct
type StatusPage struct {
	ID        int64     `json:"id"`
	Slug      string    `json:"slug"`
	Title     string    `json:"title"`
	GroupID   *string   `json:"groupId"` // Nullable
	Public    bool      `json:"public"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
}

// GetStatusPages returns all status page configs
func (s *Store) GetStatusPages() ([]StatusPage, error) {
	rows, err := s.db.Query("SELECT id, slug, title, group_id, public, enabled, created_at FROM status_pages")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var pages []StatusPage
	for rows.Next() {
		var p StatusPage
		var groupID sql.NullString
		if err := rows.Scan(&p.ID, &p.Slug, &p.Title, &groupID, &p.Public, &p.Enabled, &p.CreatedAt); err != nil {
			return nil, err
		}
		if groupID.Valid {
			s := groupID.String
			p.GroupID = &s
		}
		pages = append(pages, p)
	}
	return pages, nil
}

// GetStatusPageBySlug returns a specific status page config
func (s *Store) GetStatusPageBySlug(slug string) (*StatusPage, error) {
	var p StatusPage
	var groupID sql.NullString
	err := s.db.QueryRow(s.rebind("SELECT id, slug, title, group_id, public, enabled, created_at FROM status_pages WHERE slug = ?"), slug).
		Scan(&p.ID, &p.Slug, &p.Title, &groupID, &p.Public, &p.Enabled, &p.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if groupID.Valid {
		s := groupID.String
		p.GroupID = &s
	}
	return &p, nil
}

// UpsertStatusPage creates or updates a status page config
func (s *Store) UpsertStatusPage(slug, title string, groupID *string, public bool, enabled bool) error {
	var err error
	if s.IsPostgres() {
		_, err = s.db.Exec(`
			INSERT INTO status_pages (slug, title, group_id, public, enabled)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT(slug) DO UPDATE SET
				title=excluded.title,
				group_id=excluded.group_id,
				public=excluded.public,
				enabled=excluded.enabled
		`, slug, title, groupID, public, enabled)
	} else {
		// SQLite: INSERT OR REPLACE (slug has UNIQUE constraint)
		_, err = s.db.Exec(`
			INSERT OR REPLACE INTO status_pages (slug, title, group_id, public, enabled)
			VALUES (?, ?, ?, ?, ?)
		`, slug, title, groupID, public, enabled)
	}
	return err
}

// ToggleStatusPage toggles the public status
func (s *Store) ToggleStatusPage(slug string, public bool) error {
	_, err := s.db.Exec(s.rebind("UPDATE status_pages SET public = ? WHERE slug = ?"), public, slug)
	return err
}

// ToggleStatusPageEnabled toggles the enabled status
func (s *Store) ToggleStatusPageEnabled(slug string, enabled bool) error {
	_, err := s.db.Exec(s.rebind("UPDATE status_pages SET enabled = ? WHERE slug = ?"), enabled, slug)
	return err
}
