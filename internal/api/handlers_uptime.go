package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/uptime"
	"github.com/go-chi/chi/v5"
)

type UptimeHandler struct {
	manager *uptime.Manager
	store   *db.Store
}

func NewUptimeHandler(manager *uptime.Manager, store *db.Store) *UptimeHandler {
	return &UptimeHandler{manager: manager, store: store}
}

func getEventsForDTO(store *db.Store, monitorID string) []MonitorEvent {
	events, err := store.GetMonitorEvents(monitorID, 10) // Get last 10 events
	if err != nil {
		return []MonitorEvent{}
	}
	var dtos []MonitorEvent
	for _, e := range events {
		dtos = append(dtos, MonitorEvent{
			ID:        strconv.Itoa(e.ID),
			Type:      e.Type,
			Message:   e.Message,
			Timestamp: e.Timestamp.Format(time.RFC3339),
		})
	}
	return dtos
}

// Response Structures matching Frontend Store
type HistoryPoint struct {
	Status     string    `json:"status"`
	Latency    int64     `json:"latency"`
	Timestamp  time.Time `json:"timestamp"`
	StatusCode int       `json:"statusCode"`
}

type MonitorDTO struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	URL       string         `json:"url"`
	Status    string         `json:"status"`
	Active    bool           `json:"active"`
	Latency   int64          `json:"latency"`
	Interval  int            `json:"interval"`
	History   []HistoryPoint `json:"history"`
	Events    []MonitorEvent `json:"events"`
	LastCheck string         `json:"lastCheck"`
}

type MonitorEvent struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

type GroupDTO struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Monitors []MonitorDTO `json:"monitors"`
}

type GroupOverviewDTO struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"` // up, down, degraded
}

type OverviewResponse struct {
	Groups []GroupOverviewDTO `json:"groups"`
}

type UptimeResponse struct {
	Groups []GroupDTO `json:"groups"`
}

// GetHistory returns all monitors grouped by group with ping history.
// @Summary      List monitors with history
// @Tags         uptime
// @Produce      json
// @Security     BearerAuth
// @Param        group_id query string false "Filter by group ID"
// @Success      200  {object} UptimeResponse
// @Failure      500  {string} string "Internal error"
// @Router       /uptime [get]
func (h *UptimeHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	// 1. Fetch Layout from DB (Groups + Monitors Metadata)
	groups, err := h.store.GetGroups()
	if err != nil {
		http.Error(w, "Failed to load groups", http.StatusInternalServerError)
		return
	}

	monitorsMeta, err := h.store.GetMonitors()
	if err != nil {
		http.Error(w, "Failed to load monitors", http.StatusInternalServerError)
		return
	}

	// 2. Map Monitors to Groups
	groupMap := make(map[string][]db.Monitor)
	for _, m := range monitorsMeta {
		groupMap[m.GroupID] = append(groupMap[m.GroupID], m)
	}

	// 3. Construct Response
	var groupDTOs []GroupDTO
	filterGroupID := r.URL.Query().Get("group_id")

	for _, g := range groups {
		if filterGroupID != "" && g.ID != filterGroupID {
			continue
		}
		monitorDTOs := []MonitorDTO{} // Ensure initialized as empty slice, not nil

		for _, meta := range groupMap[g.ID] {
			// Get Live Status from Manager
			task := h.manager.GetMonitor(meta.ID)

			statusStr := "down" // Default if not running
			latency := int64(0)
			lastCheck := "Never"
			var historyPoints []HistoryPoint

			if task != nil {
				// It is running
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
					lastCheck = last.Timestamp.Format(time.RFC3339)

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
					// Running but no history yet?
					statusStr = "up" // Optimistic?
				}
			} else {
				// Not running (inactive or manager hasn't synced yet)
				if !meta.Active {
					statusStr = "paused" // Or "down"
				}
			}

			monitorDTOs = append(monitorDTOs, MonitorDTO{
				ID:        meta.ID,
				Name:      meta.Name,
				URL:       meta.URL,
				Status:    statusStr,
				Active:    meta.Active,
				Latency:   latency,
				Interval:  meta.Interval,
				History:   historyPoints,
				LastCheck: lastCheck,
				Events:    getEventsForDTO(h.store, meta.ID),
			})
		}

		groupDTOs = append(groupDTOs, GroupDTO{
			ID:       g.ID,
			Name:     g.Name,
			Monitors: monitorDTOs,
		})
	}

	resp := UptimeResponse{
		Groups: groupDTOs,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GetMonitorUptime returns uptime percentages for 24h, 7d, and 30d.
// @Summary      Get monitor uptime stats
// @Tags         uptime
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string true "Monitor ID"
// @Success      200  {object} object{uptime24h=number,uptime7d=number,uptime30d=number}
// @Failure      400  {string} string "ID required"
// @Failure      500  {string} string "Failed to calculate stats"
// @Router       /monitors/{id}/uptime [get]
func (h *UptimeHandler) GetMonitorUptime(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	u24, u7, u30, err := h.store.GetUptimeStats(id)
	if err != nil {
		http.Error(w, "Failed to calculate stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := map[string]float64{
		"uptime24h": u24,
		"uptime7d":  u7,
		"uptime30d": u30,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
// GetMonitorLatency returns latency datapoints over a time range.
// @Summary      Get monitor latency history
// @Tags         uptime
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string true  "Monitor ID"
// @Param        range query string false "Time range: 1h, 24h, 7d, 30d (default 24h)"
// @Success      200   {array} db.CheckResult
// @Failure      400   {string} string "ID required"
// @Failure      500   {string} string "Failed to fetch latency stats"
// @Router       /monitors/{id}/latency [get]
func (h *UptimeHandler) GetMonitorLatency(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	rangeStr := r.URL.Query().Get("range")
	var hours int
	switch rangeStr {
	case "1h":
		hours = 1
	case "24h":
		hours = 24
	case "7d":
		hours = 168
	case "30d":
		hours = 720
	default:
		hours = 24
	}

	points, err := h.store.GetLatencyStats(id, hours)
	if err != nil {
		http.Error(w, "Failed to fetch latency stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(points)
}

// GetOverview returns a high-level status for each group.
// @Summary      Dashboard overview
// @Tags         uptime
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object} OverviewResponse
// @Failure      500  {string} string "Internal error"
// @Router       /overview [get]
func (h *UptimeHandler) GetOverview(w http.ResponseWriter, r *http.Request) {
	groups, err := h.store.GetGroups()
	if err != nil {
		http.Error(w, "Failed to load groups", http.StatusInternalServerError)
		return
	}

	monitorsMeta, err := h.store.GetMonitors()
	if err != nil {
		http.Error(w, "Failed to load monitors", http.StatusInternalServerError)
		return
	}

	groupMap := make(map[string][]db.Monitor)
	for _, m := range monitorsMeta {
		groupMap[m.GroupID] = append(groupMap[m.GroupID], m)
	}

	var overview []GroupOverviewDTO
	threshold := h.manager.GetLatencyThreshold()

	for _, g := range groups {
		monitors := groupMap[g.ID]
		status := "up" // Default to up if no monitors or all up

		if h.manager.IsGroupInMaintenance(g.ID) {
			status = "maintenance"
		} else {
			if len(monitors) == 0 {
				status = "up"
			} else {
				anyDown := false
				anyDegraded := false

				for _, m := range monitors {
					if !m.Active {
						continue
					}
					task := h.manager.GetMonitor(m.ID)
					if task != nil {
						isUp, latency, hasHistory, isDegraded := task.GetLastStatus()
						if hasHistory && !isUp {
							anyDown = true
							break // Critical priority
						}
						if hasHistory && isUp && (isDegraded || latency > threshold) {
							anyDegraded = true
						}
					}
				}

				if anyDown {
					status = "down"
				} else if anyDegraded {
					status = "degraded"
				}
			}
		}

		overview = append(overview, GroupOverviewDTO{
			ID:     g.ID,
			Name:   g.Name,
			Status: status,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(OverviewResponse{Groups: overview})
}
