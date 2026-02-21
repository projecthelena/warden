package db

import (
	"database/sql"
	"time"
)

// StatusPage Struct
type StatusPage struct {
	ID                    int64     `json:"id"`
	Slug                  string    `json:"slug"`
	Title                 string    `json:"title"`
	GroupID               *string   `json:"groupId"` // Nullable
	Public                bool      `json:"public"`
	Enabled               bool      `json:"enabled"`
	CreatedAt             time.Time `json:"createdAt"`
	Description           string    `json:"description"`
	LogoURL               string    `json:"logoUrl"`
	AccentColor           string    `json:"accentColor"`
	Theme                 string    `json:"theme"` // 'light', 'dark', 'system'
	ShowUptimeBars        bool      `json:"showUptimeBars"`
	ShowUptimePercentage  bool      `json:"showUptimePercentage"`
	ShowIncidentHistory   bool      `json:"showIncidentHistory"`
}

// GetStatusPages returns all status page configs
func (s *Store) GetStatusPages() ([]StatusPage, error) {
	rows, err := s.db.Query(`SELECT id, slug, title, group_id, public, enabled, created_at,
		COALESCE(description, ''), COALESCE(logo_url, ''), COALESCE(accent_color, ''), COALESCE(theme, 'system'),
		COALESCE(show_uptime_bars, TRUE), COALESCE(show_uptime_percentage, TRUE), COALESCE(show_incident_history, TRUE)
		FROM status_pages`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var pages []StatusPage
	for rows.Next() {
		var p StatusPage
		var groupID sql.NullString
		if err := rows.Scan(&p.ID, &p.Slug, &p.Title, &groupID, &p.Public, &p.Enabled, &p.CreatedAt,
			&p.Description, &p.LogoURL, &p.AccentColor, &p.Theme,
			&p.ShowUptimeBars, &p.ShowUptimePercentage, &p.ShowIncidentHistory); err != nil {
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
	err := s.db.QueryRow(s.rebind(`SELECT id, slug, title, group_id, public, enabled, created_at,
		COALESCE(description, ''), COALESCE(logo_url, ''), COALESCE(accent_color, ''), COALESCE(theme, 'system'),
		COALESCE(show_uptime_bars, TRUE), COALESCE(show_uptime_percentage, TRUE), COALESCE(show_incident_history, TRUE)
		FROM status_pages WHERE slug = ?`), slug).
		Scan(&p.ID, &p.Slug, &p.Title, &groupID, &p.Public, &p.Enabled, &p.CreatedAt,
			&p.Description, &p.LogoURL, &p.AccentColor, &p.Theme,
			&p.ShowUptimeBars, &p.ShowUptimePercentage, &p.ShowIncidentHistory)
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

// StatusPageInput contains all fields for creating/updating a status page
type StatusPageInput struct {
	Slug                 string
	Title                string
	GroupID              *string
	Public               bool
	Enabled              bool
	Description          string
	LogoURL              string
	AccentColor          string
	Theme                string
	ShowUptimeBars       bool
	ShowUptimePercentage bool
	ShowIncidentHistory  bool
}

// UpsertStatusPage creates or updates a status page config
func (s *Store) UpsertStatusPage(slug, title string, groupID *string, public bool, enabled bool) error {
	return s.UpsertStatusPageFull(StatusPageInput{
		Slug:                 slug,
		Title:                title,
		GroupID:              groupID,
		Public:               public,
		Enabled:              enabled,
		Description:          "",
		LogoURL:              "",
		AccentColor:          "",
		Theme:                "system",
		ShowUptimeBars:       true,
		ShowUptimePercentage: true,
		ShowIncidentHistory:  true,
	})
}

// UpsertStatusPageFull creates or updates a status page config with all fields
func (s *Store) UpsertStatusPageFull(input StatusPageInput) error {
	var err error
	if s.IsPostgres() {
		_, err = s.db.Exec(`
			INSERT INTO status_pages (slug, title, group_id, public, enabled, description, logo_url, accent_color, theme, show_uptime_bars, show_uptime_percentage, show_incident_history)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			ON CONFLICT(slug) DO UPDATE SET
				title=excluded.title,
				group_id=excluded.group_id,
				public=excluded.public,
				enabled=excluded.enabled,
				description=excluded.description,
				logo_url=excluded.logo_url,
				accent_color=excluded.accent_color,
				theme=excluded.theme,
				show_uptime_bars=excluded.show_uptime_bars,
				show_uptime_percentage=excluded.show_uptime_percentage,
				show_incident_history=excluded.show_incident_history
		`, input.Slug, input.Title, input.GroupID, input.Public, input.Enabled,
			input.Description, input.LogoURL, input.AccentColor, input.Theme,
			input.ShowUptimeBars, input.ShowUptimePercentage, input.ShowIncidentHistory)
	} else {
		// SQLite: INSERT OR REPLACE (slug has UNIQUE constraint)
		_, err = s.db.Exec(`
			INSERT OR REPLACE INTO status_pages (slug, title, group_id, public, enabled, description, logo_url, accent_color, theme, show_uptime_bars, show_uptime_percentage, show_incident_history)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, input.Slug, input.Title, input.GroupID, input.Public, input.Enabled,
			input.Description, input.LogoURL, input.AccentColor, input.Theme,
			input.ShowUptimeBars, input.ShowUptimePercentage, input.ShowIncidentHistory)
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
