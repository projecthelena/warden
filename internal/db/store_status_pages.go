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
	CreatedAt time.Time `json:"createdAt"`
}

// GetStatusPages returns all status page configs
func (s *Store) GetStatusPages() ([]StatusPage, error) {
	rows, err := s.db.Query("SELECT id, slug, title, group_id, public, created_at FROM status_pages")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var pages []StatusPage
	for rows.Next() {
		var p StatusPage
		var groupID sql.NullString
		if err := rows.Scan(&p.ID, &p.Slug, &p.Title, &groupID, &p.Public, &p.CreatedAt); err != nil {
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
	err := s.db.QueryRow("SELECT id, slug, title, group_id, public, created_at FROM status_pages WHERE slug = ?", slug).
		Scan(&p.ID, &p.Slug, &p.Title, &groupID, &p.Public, &p.CreatedAt)
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
func (s *Store) UpsertStatusPage(slug, title string, groupID *string, public bool) error {
	_, err := s.db.Exec(`
		INSERT INTO status_pages (slug, title, group_id, public)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(slug) DO UPDATE SET
			title=excluded.title,
			group_id=excluded.group_id,
			public=excluded.public
	`, slug, title, groupID, public)
	return err
}

// ToggleStatusPage toggles the public status
func (s *Store) ToggleStatusPage(slug string, public bool) error {
	_, err := s.db.Exec("UPDATE status_pages SET public = ? WHERE slug = ?", public, slug)
	return err
}
