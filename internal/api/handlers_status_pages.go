package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/uptime"
	"github.com/go-chi/chi/v5"
)

var hexColorRegex = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

type StatusPageHandler struct {
	store   *db.Store
	manager *uptime.Manager
	auth    *AuthHandler
}

func NewStatusPageHandler(store *db.Store, manager *uptime.Manager, auth *AuthHandler) *StatusPageHandler {
	return &StatusPageHandler{store: store, manager: manager, auth: auth}
}

// GetAll returns all status page configurations merged with groups.
// @Summary      List status pages
// @Tags         status-pages
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object} object{pages=[]object{slug=string,title=string,groupId=string,public=bool}}
// @Failure      500  {object} object{error=string}
// @Router       /status-pages [get]
func (h *StatusPageHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	// 1. Fetch Configured Pages
	pages, err := h.store.GetStatusPages()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch status pages")
		return
	}

	// 2. Fetch All Groups
	groups, err := h.store.GetGroups()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch groups")
		return
	}

	// 3. Construct Unified List
	type StatusPageDTO struct {
		Slug    string  `json:"slug"`
		Title   string  `json:"title"`
		GroupID *string `json:"groupId"`
		Public  bool    `json:"public"`
		Enabled bool    `json:"enabled"`
	}

	var result []StatusPageDTO

	// Map configured pages by GroupID (and handle Global "all")
	configMap := make(map[string]db.StatusPage)
	var globalPage *db.StatusPage

	for _, p := range pages {
		if p.Slug == "all" {
			global := p // Copy
			globalPage = &global
		} else if p.GroupID != nil {
			configMap[*p.GroupID] = p
		}
	}

	// A. Global Page
	globalPublic := false
	globalEnabled := false
	if globalPage != nil {
		globalPublic = globalPage.Public
		globalEnabled = globalPage.Enabled
	}
	result = append(result, StatusPageDTO{
		Slug:    "all",
		Title:   "Global Status",
		GroupID: nil,
		Public:  globalPublic,
		Enabled: globalEnabled,
	})

	// B. Group Pages
	for _, g := range groups {
		slug := strings.TrimPrefix(g.ID, "g-") // default slug (clean)
		title := g.Name
		public := false
		enabled := false

		if cfg, ok := configMap[g.ID]; ok {
			slug = cfg.Slug
			title = cfg.Title
			public = cfg.Public
			enabled = cfg.Enabled
		}

		result = append(result, StatusPageDTO{
			Slug:    slug,
			Title:   title,
			GroupID: &g.ID,
			Public:  public,
			Enabled: enabled,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"pages": result})
}

// Toggle enables or disables a public status page.
// @Summary      Toggle status page
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        slug path string true "Status page slug"
// @Param        body body object{public=bool,title=string,groupId=string} true "Toggle payload"
// @Success      200  {object} object{message=string}
// @Failure      400  {object} object{error=string} "Invalid request"
// @Router       /status-pages/{slug} [patch]
func (h *StatusPageHandler) Toggle(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	var req struct {
		Public               bool    `json:"public"`
		Enabled              bool    `json:"enabled"`
		Title                string  `json:"title"`
		GroupID              *string `json:"groupId"`
		Description          *string `json:"description"`
		LogoURL              *string `json:"logoUrl"`
		AccentColor          *string `json:"accentColor"`
		Theme                *string `json:"theme"`
		ShowUptimeBars       *bool   `json:"showUptimeBars"`
		ShowUptimePercentage *bool   `json:"showUptimePercentage"`
		ShowIncidentHistory  *bool   `json:"showIncidentHistory"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	// Validate accent_color if provided
	accentColor := ""
	if req.AccentColor != nil && *req.AccentColor != "" {
		if !hexColorRegex.MatchString(*req.AccentColor) {
			writeError(w, http.StatusBadRequest, "invalid accent color format (must be #RRGGBB)")
			return
		}
		accentColor = *req.AccentColor
	}

	// Validate theme if provided
	theme := "system"
	if req.Theme != nil && *req.Theme != "" {
		if *req.Theme != "light" && *req.Theme != "dark" && *req.Theme != "system" {
			writeError(w, http.StatusBadRequest, "invalid theme (must be light, dark, or system)")
			return
		}
		theme = *req.Theme
	}

	// Validate logo_url if provided (URL or data:image/* URI)
	logoURL := ""
	if req.LogoURL != nil && *req.LogoURL != "" {
		logo := *req.LogoURL
		if !strings.HasPrefix(logo, "http://") && !strings.HasPrefix(logo, "https://") && !strings.HasPrefix(logo, "data:image/") {
			writeError(w, http.StatusBadRequest, "invalid logo URL (must be http/https URL or data:image/* URI)")
			return
		}
		logoURL = logo
	}

	// Get existing page to preserve defaults
	existing, _ := h.store.GetStatusPageBySlug(slug)

	// Build input with defaults
	input := db.StatusPageInput{
		Slug:                 slug,
		Title:                req.Title,
		GroupID:              req.GroupID,
		Public:               req.Public,
		Enabled:              req.Enabled,
		Description:          "",
		LogoURL:              logoURL,
		AccentColor:          accentColor,
		Theme:                theme,
		ShowUptimeBars:       true,
		ShowUptimePercentage: true,
		ShowIncidentHistory:  true,
	}

	// Apply existing values as defaults
	if existing != nil {
		input.Description = existing.Description
		if logoURL == "" && req.LogoURL == nil {
			input.LogoURL = existing.LogoURL
		}
		if accentColor == "" && req.AccentColor == nil {
			input.AccentColor = existing.AccentColor
		}
		if req.Theme == nil {
			input.Theme = existing.Theme
		}
		input.ShowUptimeBars = existing.ShowUptimeBars
		input.ShowUptimePercentage = existing.ShowUptimePercentage
		input.ShowIncidentHistory = existing.ShowIncidentHistory
	}

	// Override with request values if provided
	if req.Description != nil {
		input.Description = *req.Description
	}
	if req.ShowUptimeBars != nil {
		input.ShowUptimeBars = *req.ShowUptimeBars
	}
	if req.ShowUptimePercentage != nil {
		input.ShowUptimePercentage = *req.ShowUptimePercentage
	}
	if req.ShowIncidentHistory != nil {
		input.ShowIncidentHistory = *req.ShowIncidentHistory
	}

	if err := h.store.UpsertStatusPageFull(input); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update status page")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "updated"})
}

// GetPublicStatus returns real-time status data for a public status page.
// @Summary      Public status page
// @Tags         status-pages
// @Produce      json
// @Param        slug path string true "Status page slug"
// @Success      200  {object} object{title=string,public=bool,groups=[]object{id=string,name=string},incidents=[]object{id=string,title=string}}
// @Failure      403  {object} object{error=string} "Status page is private"
// @Failure      404  {object} object{error=string} "Status page not found"
// @Router       /s/{slug} [get]
func (h *StatusPageHandler) GetPublicStatus(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	// 1. Check Config
	page, err := h.store.GetStatusPageBySlug(slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "error fetching status page")
		return
	}
	if page == nil || !page.Enabled {
		writeError(w, http.StatusNotFound, "status page not found")
		return
	}
	if !page.Public {
		if !h.auth.IsAuthenticated(r) {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
	}

	// 2. Fetch Layout from DB (Groups + Monitors Metadata)
	groups, err := h.store.GetGroups()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load groups")
		return
	}

	monitorsMeta, err := h.store.GetMonitors()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load monitors")
		return
	}

	// 3. Filter Groups if Status Page scans specific Group
	var targetGroups []db.Group
	if page.GroupID != nil {
		// Only include the specific group
		for _, g := range groups {
			if g.ID == *page.GroupID {
				targetGroups = append(targetGroups, g)
				break
			}
		}
		if len(targetGroups) == 0 {
			// Group might have been deleted? Return empty
			writeJSON(w, http.StatusOK, map[string]any{
				"title":  page.Title,
				"groups": []any{},
			})
			return
		}
	} else {
		// All groups
		targetGroups = groups
	}

	// 4. Map Monitors to Groups
	groupMap := make(map[string][]db.Monitor)
	for _, m := range monitorsMeta {
		groupMap[m.GroupID] = append(groupMap[m.GroupID], m)
	}

	// 5. Construct Response (Reusing Logic from UptimeHandler)
	type MonitorDTO struct {
		ID             string              `json:"id"`
		Name           string              `json:"name"`
		URL            string              `json:"url"`
		Status         string              `json:"status"`
		Latency        int64               `json:"latency"`
		History        []HistoryPoint      `json:"history"`
		LastCheck      string              `json:"lastCheck"`
		UptimeDays     []db.DailyUptimeStat `json:"uptimeDays"`
		OverallUptime  float64             `json:"overallUptime"`
	}

	type GroupDTO struct {
		ID       string       `json:"id"`
		Name     string       `json:"name"`
		Monitors []MonitorDTO `json:"monitors"`
	}

	groupDTOs := []GroupDTO{}

	for _, g := range targetGroups {
		monitorDTOs := []MonitorDTO{}

		for _, meta := range groupMap[g.ID] {
			// Get Live Status from Manager
			task := h.manager.GetMonitor(meta.ID)

			statusStr := "down" // Default if not running
			latency := int64(0)
			lastCheck := "Never"
			var historyPoints []HistoryPoint

			if task != nil {
				history := task.GetHistory()
				if len(history) > 0 {
					last := history[len(history)-1]
					threshold := h.manager.GetLatencyThreshold()
					if last.IsUp {
						statusStr = "up"
						if last.Latency > threshold {
							statusStr = "degraded"
						}
					}
					latency = last.Latency
					lastCheck = last.Timestamp.Format("15:04:05")

					for _, h := range history {
						s := "down"
						if h.IsUp {
							s = "up"
							if h.Latency > threshold {
								s = "degraded"
							}
						}
						historyPoints = append(historyPoints, HistoryPoint{
							Status:     s,
							Latency:    h.Latency,
							Timestamp:  h.Timestamp,
							StatusCode: h.StatusCode,
						})
					}
				} else {
					statusStr = "up" // Optimistic or "pending"
				}
			} else {
				if !meta.Active {
					statusStr = "paused"
				}
			}

			// Fetch 90-day daily uptime stats from DB
			uptimeDays, _ := h.store.GetDailyUptimeStats(meta.ID, 90)
			if uptimeDays == nil {
				uptimeDays = []db.DailyUptimeStat{}
			}

			// Compute overall uptime from the daily stats
			var totalChecks, totalUp int
			for _, d := range uptimeDays {
				totalChecks += d.Total
				totalUp += d.Up
			}
			overallUptime := 100.0
			if totalChecks > 0 {
				overallUptime = (float64(totalUp) / float64(totalChecks)) * 100.0
			}

			monitorDTOs = append(monitorDTOs, MonitorDTO{
				ID:            meta.ID,
				Name:          meta.Name,
				URL:           meta.URL,
				Status:        statusStr,
				Latency:       latency,
				History:       historyPoints,
				LastCheck:     lastCheck,
				UptimeDays:    uptimeDays,
				OverallUptime: overallUptime,
			})
		}

		// Only add groups that have monitors or return empty groups too?
		// Let's return all groups for now.
		groupDTOs = append(groupDTOs, GroupDTO{
			ID:       g.ID,
			Name:     g.Name,
			Monitors: monitorDTOs,
		})
	}

	// 6. Fetch Incidents and Outages
	type IncidentUpdateDTO struct {
		Status    string    `json:"status"`
		Message   string    `json:"message"`
		CreatedAt time.Time `json:"createdAt"`
	}

	type IncidentResponseDTO struct {
		ID             string              `json:"id"`
		Title          string              `json:"title"`
		Description    string              `json:"description"`
		Type           string              `json:"type"`
		Severity       string              `json:"severity"`
		Status         string              `json:"status"`
		StartTime      time.Time           `json:"startTime"`
		EndTime        *time.Time          `json:"endTime,omitempty"`
		AffectedGroups []string            `json:"affectedGroups"`
		Source         string              `json:"source,omitempty"`
		Duration       string              `json:"duration,omitempty"`
		Updates        []IncidentUpdateDTO `json:"updates,omitempty"`
	}

	activeIncidents := []IncidentResponseDTO{}

	// Fetch all incidents first to build a set of promoted outage IDs
	allIncidents, _ := h.store.GetIncidents(time.Time{})
	promotedOutageIDs := make(map[int64]bool)
	for _, inc := range allIncidents {
		if inc.OutageID != nil {
			promotedOutageIDs[*inc.OutageID] = true
		}
	}

	// A. Auto-detected Outages (only show if not already promoted to an incident)
	activeOutages, err := h.store.GetActiveOutages()
	if err == nil {
		for _, o := range activeOutages {
			// Skip outages that have been promoted to an incident
			if promotedOutageIDs[o.ID] {
				continue
			}
			// Filter by Group if needed
			if page.GroupID != nil && o.GroupID != *page.GroupID {
				continue
			}

			activeIncidents = append(activeIncidents, IncidentResponseDTO{
				ID:             "auto-" + o.MonitorID, // Temporary ID
				Title:          "Service Disruption: " + o.MonitorName,
				Description:    o.Summary,
				Type:           "incident",
				Severity:       "critical",
				Status:         "investigating",
				StartTime:      o.StartTime,
				AffectedGroups: []string{o.GroupID},
				Source:         "auto",
			})
		}
	}

	// B. Active Manual/Promoted Incidents (not resolved/completed, must be public)
	for _, inc := range allIncidents {
		if inc.Status == "completed" || inc.Status == "resolved" {
			continue
		}
		// Skip private incidents on public status page
		if !inc.Public {
			continue
		}

		// Parse Groups
		var mappedGroups []string
		if inc.AffectedGroups != "" {
			_ = json.Unmarshal([]byte(inc.AffectedGroups), &mappedGroups)
		}

		// Filter by Group
		if page.GroupID != nil {
			affected := false
			if len(mappedGroups) == 0 {
				// Assume global?
			} else {
				for _, gID := range mappedGroups {
					if gID == *page.GroupID {
						affected = true
						break
					}
				}
			}
			if !affected {
				continue
			}
		}

		// Get updates for timeline
		var updateDTOs []IncidentUpdateDTO
		updates, _ := h.store.GetIncidentUpdates(inc.ID)
		for _, u := range updates {
			updateDTOs = append(updateDTOs, IncidentUpdateDTO{
				Status:    u.Status,
				Message:   u.Message,
				CreatedAt: u.CreatedAt,
			})
		}

		source := inc.Source
		if source == "" {
			source = "manual"
		}

		activeIncidents = append(activeIncidents, IncidentResponseDTO{
			ID:             inc.ID,
			Title:          inc.Title,
			Description:    inc.Description,
			Type:           inc.Type,
			Severity:       inc.Severity,
			Status:         inc.Status,
			StartTime:      inc.StartTime,
			EndTime:        inc.EndTime,
			AffectedGroups: mappedGroups,
			Source:         source,
			Updates:        updateDTOs,
		})
	}

	// 7. Fetch Past Incidents (public, resolved, last 14 days)
	pastIncidents := []IncidentResponseDTO{}
	since := time.Now().Add(-14 * 24 * time.Hour)
	publicResolved, err := h.store.GetPublicResolvedIncidents(since)
	if err == nil {
		for _, inc := range publicResolved {
			// Parse Groups
			var mappedGroups []string
			if inc.AffectedGroups != "" {
				_ = json.Unmarshal([]byte(inc.AffectedGroups), &mappedGroups)
			}

			// Filter by Group if this is a group-specific status page
			if page.GroupID != nil {
				affected := false
				if len(mappedGroups) == 0 {
					// Global incident - show on all pages
					affected = true
				} else {
					for _, gID := range mappedGroups {
						if gID == *page.GroupID {
							affected = true
							break
						}
					}
				}
				if !affected {
					continue
				}
			}

			// Get updates for timeline
			var updateDTOs []IncidentUpdateDTO
			updates, _ := h.store.GetIncidentUpdates(inc.ID)
			for _, u := range updates {
				updateDTOs = append(updateDTOs, IncidentUpdateDTO{
					Status:    u.Status,
					Message:   u.Message,
					CreatedAt: u.CreatedAt,
				})
			}

			// Calculate duration
			var duration string
			if inc.EndTime != nil {
				d := inc.EndTime.Sub(inc.StartTime)
				if d < time.Hour {
					duration = formatDurationMinutes(d)
				} else {
					duration = formatDurationHours(d)
				}
			}

			source := inc.Source
			if source == "" {
				source = "manual"
			}

			pastIncidents = append(pastIncidents, IncidentResponseDTO{
				ID:             inc.ID,
				Title:          inc.Title,
				Description:    inc.Description,
				Type:           inc.Type,
				Severity:       inc.Severity,
				Status:         inc.Status,
				StartTime:      inc.StartTime,
				EndTime:        inc.EndTime,
				AffectedGroups: mappedGroups,
				Source:         source,
				Duration:       duration,
				Updates:        updateDTOs,
			})
		}
	}

	// Build config object for public page
	config := map[string]any{
		"description":          page.Description,
		"logoUrl":              page.LogoURL,
		"accentColor":          page.AccentColor,
		"theme":                page.Theme,
		"showUptimeBars":       page.ShowUptimeBars,
		"showUptimePercentage": page.ShowUptimePercentage,
		"showIncidentHistory":  page.ShowIncidentHistory,
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"title":         page.Title,
		"public":        page.Public,
		"groups":        groupDTOs,
		"incidents":     activeIncidents,
		"pastIncidents": pastIncidents,
		"config":        config,
	})
}

func formatDurationMinutes(d time.Duration) string {
	mins := int(d.Minutes())
	if mins < 1 {
		return "<1m"
	}
	return strings.TrimSpace(strings.Replace(d.Truncate(time.Minute).String(), "0s", "", 1))
}

func formatDurationHours(d time.Duration) string {
	mins := int(d.Minutes()) % 60
	if mins == 0 {
		return strings.TrimSpace(d.Truncate(time.Hour).String())
	}
	return strings.TrimSpace(d.Truncate(time.Minute).String())
}

// GetRSSFeed returns an RSS 2.0 feed of recent incidents for a public status page.
// @Summary      RSS feed for status page
// @Tags         status-pages
// @Produce      application/rss+xml
// @Param        slug path string true "Status page slug"
// @Success      200  {string} string "RSS 2.0 XML feed"
// @Failure      404  {object} object{error=string} "Status page not found"
// @Router       /s/{slug}/rss [get]
func (h *StatusPageHandler) GetRSSFeed(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	// 1. Check Config
	page, err := h.store.GetStatusPageBySlug(slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "error fetching status page")
		return
	}
	if page == nil || !page.Enabled {
		writeError(w, http.StatusNotFound, "status page not found")
		return
	}
	if !page.Public {
		writeError(w, http.StatusNotFound, "status page not found")
		return
	}

	// 2. Build base URL from request
	scheme := "https"
	if r.TLS == nil {
		// Check for X-Forwarded-Proto header (common with reverse proxies)
		if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
			scheme = proto
		} else {
			scheme = "http"
		}
	}
	baseURL := scheme + "://" + r.Host

	// 3. Fetch recent public incidents (last 30 days)
	since := time.Now().Add(-30 * 24 * time.Hour)
	allIncidents, _ := h.store.GetIncidents(since)

	// Filter to public incidents only
	var feedIncidents []db.Incident
	for _, inc := range allIncidents {
		if !inc.Public {
			continue
		}

		// Filter by group if this is a group-specific status page
		if page.GroupID != nil {
			var mappedGroups []string
			if inc.AffectedGroups != "" {
				_ = json.Unmarshal([]byte(inc.AffectedGroups), &mappedGroups)
			}

			affected := false
			if len(mappedGroups) == 0 {
				// Global incident - show on all pages
				affected = true
			} else {
				for _, gID := range mappedGroups {
					if gID == *page.GroupID {
						affected = true
						break
					}
				}
			}
			if !affected {
				continue
			}
		}

		feedIncidents = append(feedIncidents, inc)
	}

	// 4. Build RSS 2.0 XML
	statusPageURL := baseURL + "/status/" + slug
	feedURL := baseURL + "/api/s/" + slug + "/rss"

	var lastBuildDate time.Time
	if len(feedIncidents) > 0 {
		lastBuildDate = feedIncidents[0].StartTime
		for _, inc := range feedIncidents {
			if inc.StartTime.After(lastBuildDate) {
				lastBuildDate = inc.StartTime
			}
		}
	} else {
		lastBuildDate = time.Now()
	}

	// Build items XML
	var itemsXML strings.Builder
	for _, inc := range feedIncidents {
		// Build description with updates
		description := inc.Description
		updates, _ := h.store.GetIncidentUpdates(inc.ID)
		if len(updates) > 0 {
			description += "\n\nUpdates:\n"
			for _, u := range updates {
				description += "- [" + u.Status + "] " + u.Message + " (" + u.CreatedAt.Format(time.RFC1123) + ")\n"
			}
		}

		// Format severity for title
		severityLabel := strings.ToUpper(inc.Severity)
		if inc.Type == "maintenance" {
			severityLabel = "MAINTENANCE"
		}

		itemsXML.WriteString("    <item>\n")
		itemsXML.WriteString("      <title>" + xmlEscape("["+severityLabel+"] "+inc.Title) + "</title>\n")
		itemsXML.WriteString("      <description>" + xmlEscape(description) + "</description>\n")
		itemsXML.WriteString("      <link>" + xmlEscape(statusPageURL+"#incident-"+inc.ID) + "</link>\n")
		itemsXML.WriteString("      <guid isPermaLink=\"false\">incident-" + inc.ID + "</guid>\n")
		itemsXML.WriteString("      <pubDate>" + inc.StartTime.Format(time.RFC1123Z) + "</pubDate>\n")
		itemsXML.WriteString("    </item>\n")
	}

	// Build full RSS feed
	rss := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">
  <channel>
    <title>` + xmlEscape(page.Title+" - Status Updates") + `</title>
    <link>` + xmlEscape(statusPageURL) + `</link>
    <description>` + xmlEscape("Status updates for "+page.Title) + `</description>
    <atom:link href="` + xmlEscape(feedURL) + `" rel="self" type="application/rss+xml"/>
    <lastBuildDate>` + lastBuildDate.Format(time.RFC1123Z) + `</lastBuildDate>
` + itemsXML.String() + `  </channel>
</rss>`

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(rss))
}

// xmlEscape escapes special XML characters
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
