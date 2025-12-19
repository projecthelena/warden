package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/clusteruptime/clusteruptime/internal/db"
	"github.com/clusteruptime/clusteruptime/internal/uptime"
	"github.com/go-chi/chi/v5"
)

type StatusPageHandler struct {
	store   *db.Store
	manager *uptime.Manager
}

func NewStatusPageHandler(store *db.Store, manager *uptime.Manager) *StatusPageHandler {
	return &StatusPageHandler{store: store, manager: manager}
}

// Admin: Get all status page configs merged with groups
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
	if globalPage != nil {
		globalPublic = globalPage.Public
	}
	result = append(result, StatusPageDTO{
		Slug:    "all",
		Title:   "Global Status",
		GroupID: nil,
		Public:  globalPublic,
	})

	// B. Group Pages
	for _, g := range groups {
		slug := strings.TrimPrefix(g.ID, "g-") // default slug (clean)
		title := g.Name
		public := false

		if cfg, ok := configMap[g.ID]; ok {
			slug = cfg.Slug
			title = cfg.Title
			public = cfg.Public
		}

		result = append(result, StatusPageDTO{
			Slug:    slug,
			Title:   title,
			GroupID: &g.ID,
			Public:  public,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"pages": result})
}

// Admin: Toggle status page
func (h *StatusPageHandler) Toggle(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	var req struct {
		Public  bool    `json:"public"`
		Title   string  `json:"title"`   // Added for Upsert
		GroupID *string `json:"groupId"` // Added for Upsert
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	// Use Upsert instead of just Update
	if err := h.store.UpsertStatusPage(slug, req.Title, req.GroupID, req.Public); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update status page")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "updated"})
}

// Public: Get status data if enabled
// This needs access to Uptime manager to get real-time data.
// We will need to inject Manager into StatusPageHandler or refactor.
// For now, let's inject Manager.
// Public: Get status data if enabled
// Public: Get status data if enabled
func (h *StatusPageHandler) GetPublicStatus(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	// 1. Check Config
	page, err := h.store.GetStatusPageBySlug(slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "error fetching status page")
		return
	}
	if page == nil {
		writeError(w, http.StatusNotFound, "status page not found")
		return
	}
	if !page.Public {
		writeError(w, http.StatusForbidden, "status page is private")
		return
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
		ID        string         `json:"id"`
		Name      string         `json:"name"`
		URL       string         `json:"url"`
		Status    string         `json:"status"`
		Latency   int64          `json:"latency"`
		History   []HistoryPoint `json:"history"`
		LastCheck string         `json:"lastCheck"`
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

			monitorDTOs = append(monitorDTOs, MonitorDTO{
				ID:        meta.ID,
				Name:      meta.Name,
				URL:       meta.URL,
				Status:    statusStr,
				Latency:   latency,
				History:   historyPoints,
				LastCheck: lastCheck,
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
	type IncidentResponseDTO struct {
		ID             string     `json:"id"`
		Title          string     `json:"title"`
		Description    string     `json:"description"`
		Type           string     `json:"type"`
		Severity       string     `json:"severity"`
		Status         string     `json:"status"`
		StartTime      time.Time  `json:"startTime"`
		EndTime        *time.Time `json:"endTime,omitempty"`
		AffectedGroups []string   `json:"affectedGroups"`
	}

	activeIncidents := []IncidentResponseDTO{}

	// A. Auto-detected Outages
	activeOutages, err := h.store.GetActiveOutages()
	if err == nil {
		for _, o := range activeOutages {
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
			})
		}
	}

	// B. Manual Incidents
	allIncidents, err := h.store.GetIncidents(time.Time{})
	if err == nil {
		for _, inc := range allIncidents {
			if inc.Status == "completed" || inc.Status == "resolved" {
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
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"title":     page.Title,
		"public":    true,
		"groups":    groupDTOs,
		"incidents": activeIncidents,
	})
}
