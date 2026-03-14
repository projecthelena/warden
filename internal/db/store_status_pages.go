package db

import (
	"database/sql"
	"time"
)

// StatusPage Struct
type StatusPage struct {
	ID                   int64     `json:"id"`
	Slug                 string    `json:"slug"`
	Title                string    `json:"title"`
	GroupID              *string   `json:"groupId"` // Nullable
	Public               bool      `json:"public"`
	Enabled              bool      `json:"enabled"`
	CreatedAt            time.Time `json:"createdAt"`
	Description          string    `json:"description"`
	LogoURL              string    `json:"logoUrl"`
	FaviconURL           string    `json:"faviconUrl"`
	AccentColor          string    `json:"accentColor"`
	Theme                string    `json:"theme"` // 'light', 'dark', 'system'
	ShowUptimeBars       bool      `json:"showUptimeBars"`
	ShowUptimePercentage bool      `json:"showUptimePercentage"`
	ShowIncidentHistory  bool      `json:"showIncidentHistory"`
	UptimeDaysRange      int       `json:"uptimeDaysRange"`
	HeaderContent     string `json:"headerContent"`     // 'logo-title', 'logo-only', 'title-only'
	HeaderAlignment   string `json:"headerAlignment"`   // 'left', 'center', 'right'
	HeaderArrangement string `json:"headerArrangement"` // 'stacked', 'inline'
}

// GetStatusPages returns all status page configs
func (s *Store) GetStatusPages() ([]StatusPage, error) {
	rows, err := s.db.Query(`SELECT id, slug, title, group_id, public, enabled, created_at,
		COALESCE(description, ''), COALESCE(logo_url, ''), COALESCE(favicon_url, ''), COALESCE(accent_color, ''), COALESCE(theme, 'system'),
		COALESCE(show_uptime_bars, TRUE), COALESCE(show_uptime_percentage, TRUE), COALESCE(show_incident_history, TRUE),
		COALESCE(uptime_days_range, 90), COALESCE(header_content, 'logo-title'), COALESCE(header_alignment, 'center'), COALESCE(header_arrangement, 'inline')
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
			&p.Description, &p.LogoURL, &p.FaviconURL, &p.AccentColor, &p.Theme,
			&p.ShowUptimeBars, &p.ShowUptimePercentage, &p.ShowIncidentHistory, &p.UptimeDaysRange,
			&p.HeaderContent, &p.HeaderAlignment, &p.HeaderArrangement); err != nil {
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
		COALESCE(description, ''), COALESCE(logo_url, ''), COALESCE(favicon_url, ''), COALESCE(accent_color, ''), COALESCE(theme, 'system'),
		COALESCE(show_uptime_bars, TRUE), COALESCE(show_uptime_percentage, TRUE), COALESCE(show_incident_history, TRUE),
		COALESCE(uptime_days_range, 90), COALESCE(header_content, 'logo-title'), COALESCE(header_alignment, 'center'), COALESCE(header_arrangement, 'inline')
		FROM status_pages WHERE slug = ?`), slug).
		Scan(&p.ID, &p.Slug, &p.Title, &groupID, &p.Public, &p.Enabled, &p.CreatedAt,
			&p.Description, &p.LogoURL, &p.FaviconURL, &p.AccentColor, &p.Theme,
			&p.ShowUptimeBars, &p.ShowUptimePercentage, &p.ShowIncidentHistory, &p.UptimeDaysRange,
			&p.HeaderContent, &p.HeaderAlignment, &p.HeaderArrangement)
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
	FaviconURL           string
	AccentColor          string
	Theme                string
	ShowUptimeBars       bool
	ShowUptimePercentage bool
	ShowIncidentHistory  bool
	UptimeDaysRange      int
	HeaderContent     string
	HeaderAlignment   string
	HeaderArrangement string
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
		FaviconURL:           "",
		AccentColor:          "",
		Theme:                "system",
		ShowUptimeBars:       true,
		ShowUptimePercentage: true,
		ShowIncidentHistory:  true,
		UptimeDaysRange:      90,
		HeaderContent:        "logo-title",
		HeaderAlignment:      "center",
		HeaderArrangement:    "stacked",
	})
}

// UpsertStatusPageFull creates or updates a status page config with all fields
func (s *Store) UpsertStatusPageFull(input StatusPageInput) error {
	var err error
	if s.IsPostgres() {
		_, err = s.db.Exec(`
			INSERT INTO status_pages (slug, title, group_id, public, enabled, description, logo_url, favicon_url, accent_color, theme, show_uptime_bars, show_uptime_percentage, show_incident_history, uptime_days_range, header_content, header_alignment, header_arrangement)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
			ON CONFLICT(slug) DO UPDATE SET
				title=excluded.title,
				group_id=excluded.group_id,
				public=excluded.public,
				enabled=excluded.enabled,
				description=excluded.description,
				logo_url=excluded.logo_url,
				favicon_url=excluded.favicon_url,
				accent_color=excluded.accent_color,
				theme=excluded.theme,
				show_uptime_bars=excluded.show_uptime_bars,
				show_uptime_percentage=excluded.show_uptime_percentage,
				show_incident_history=excluded.show_incident_history,
				uptime_days_range=excluded.uptime_days_range,
				header_content=excluded.header_content,
				header_alignment=excluded.header_alignment,
				header_arrangement=excluded.header_arrangement
		`, input.Slug, input.Title, input.GroupID, input.Public, input.Enabled,
			input.Description, input.LogoURL, input.FaviconURL, input.AccentColor, input.Theme,
			input.ShowUptimeBars, input.ShowUptimePercentage, input.ShowIncidentHistory, input.UptimeDaysRange,
			input.HeaderContent, input.HeaderAlignment, input.HeaderArrangement)
	} else {
		// SQLite: INSERT OR REPLACE (slug has UNIQUE constraint)
		_, err = s.db.Exec(`
			INSERT OR REPLACE INTO status_pages (slug, title, group_id, public, enabled, description, logo_url, favicon_url, accent_color, theme, show_uptime_bars, show_uptime_percentage, show_incident_history, uptime_days_range, header_content, header_alignment, header_arrangement)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, input.Slug, input.Title, input.GroupID, input.Public, input.Enabled,
			input.Description, input.LogoURL, input.FaviconURL, input.AccentColor, input.Theme,
			input.ShowUptimeBars, input.ShowUptimePercentage, input.ShowIncidentHistory, input.UptimeDaysRange,
			input.HeaderContent, input.HeaderAlignment, input.HeaderArrangement)
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
