package api

import (
	"encoding/json"
	"net/http"

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

// Admin: Get all status page configs
func (h *StatusPageHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	pages, err := h.store.GetStatusPages()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch status pages")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pages": pages})
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

	writeJSON(w, http.StatusOK, map[string]any{
		"title":     page.Title,
		"public":    true,
		"groups":    groupDTOs,
		"incidents": []any{}, // TODO: Fetch Incidents
	})
}
