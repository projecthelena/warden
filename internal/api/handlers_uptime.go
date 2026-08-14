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
		dto := MonitorEvent{
			ID:           strconv.Itoa(e.ID),
			Type:         e.Type,
			Message:      e.Message,
			Timestamp:    e.Timestamp.Format(time.RFC3339),
			StatusCode:   e.StatusCode,
			Latency:      e.Latency,
			ErrorMessage: e.ErrorMessage,
			ResponseBody: e.ResponseBody,
		}
		if e.ResponseHeaders != nil && *e.ResponseHeaders != "" {
			var headers map[string]string
			if err := json.Unmarshal([]byte(*e.ResponseHeaders), &headers); err == nil {
				dto.ResponseHeaders = headers
			}
		}
		dtos = append(dtos, dto)
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
	ID                      string            `json:"id"`
	Name                    string            `json:"name"`
	Type                    string            `json:"type"`
	URL                     string            `json:"url"`
	Status                  string            `json:"status"`
	Active                  bool              `json:"active"`
	Latency                 int64             `json:"latency"`
	Interval                int               `json:"interval"`
	History                 []HistoryPoint    `json:"history"`
	Events                  []MonitorEvent    `json:"events"`
	LastCheck               string            `json:"lastCheck"`
	ConfirmationThreshold   *int              `json:"confirmationThreshold,omitempty"`
	NotificationCooldownMin *int              `json:"notificationCooldownMinutes,omitempty"`
	LatencyThreshold        *int              `json:"latencyThreshold,omitempty"`
	RequestConfig           *db.RequestConfig `json:"requestConfig,omitempty"`
}

type MonitorEvent struct {
	ID              string            `json:"id"`
	Type            string            `json:"type"`
	Message         string            `json:"message"`
	Timestamp       string            `json:"timestamp"`
	StatusCode      *int              `json:"statusCode,omitempty"`
	Latency         *int64            `json:"latency,omitempty"`
	ErrorMessage    *string           `json:"errorMessage,omitempty"`
	ResponseBody    *string           `json:"responseBody,omitempty"`
	ResponseHeaders map[string]string `json:"responseHeaders,omitempty"`
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
					threshold := task.GetLatencyThreshold()
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
				ID:                      meta.ID,
				Name:                    meta.Name,
				Type:                    db.NormalizeMonitorType(meta.Type),
				URL:                     meta.URL,
				Status:                  statusStr,
				Active:                  meta.Active,
				Latency:                 latency,
				Interval:                meta.Interval,
				History:                 historyPoints,
				LastCheck:               lastCheck,
				Events:                  getEventsForDTO(h.store, meta.ID),
				ConfirmationThreshold:   meta.ConfirmationThreshold,
				NotificationCooldownMin: meta.NotificationCooldownMin,
				LatencyThreshold:        meta.LatencyThreshold,
				RequestConfig:           meta.RequestConfig,
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

// MonitorEventDTO is the enriched event payload returned to the dashboard drill-down view.
type MonitorEventDTO struct {
	ID              string             `json:"id"`
	Type            string             `json:"type"`
	Message         string             `json:"message"`
	Timestamp       string             `json:"timestamp"`
	StatusCode      *int               `json:"statusCode,omitempty"`
	Latency         *int64             `json:"latency,omitempty"`
	ErrorMessage    *string            `json:"errorMessage,omitempty"`
	ResponseBody    *string            `json:"responseBody,omitempty"`
	ResponseHeaders map[string]string  `json:"responseHeaders,omitempty"`
}

func toEventDTO(e db.MonitorEvent) MonitorEventDTO {
	dto := MonitorEventDTO{
		ID:           strconv.Itoa(e.ID),
		Type:         e.Type,
		Message:      e.Message,
		Timestamp:    e.Timestamp.Format(time.RFC3339),
		StatusCode:   e.StatusCode,
		Latency:      e.Latency,
		ErrorMessage: e.ErrorMessage,
		ResponseBody: e.ResponseBody,
	}
	if e.ResponseHeaders != nil && *e.ResponseHeaders != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(*e.ResponseHeaders), &headers); err == nil {
			dto.ResponseHeaders = headers
		}
	}
	return dto
}

// parseDateParam parses YYYY-MM-DD into a [start, end) day window in UTC. Returns the empty
// time pair when the input is blank, signaling "no date filter".
func parseDateParam(s string) (time.Time, time.Time, error) {
	if s == "" {
		return time.Time{}, time.Time{}, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	start := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return start, start.Add(24 * time.Hour), nil
}

// GetMonitorEvents returns enriched events for a single monitor, optionally filtered by date.
// @Summary      Get enriched events for a monitor
// @Tags         uptime
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string true  "Monitor ID"
// @Param        date  query string false "Date filter YYYY-MM-DD (UTC). If omitted and from/to are also omitted, returns the most recent N events."
// @Param        from  query string false "Range start (RFC3339). Used with `to` to drill into an outage window."
// @Param        to    query string false "Range end (RFC3339). Used with `from`."
// @Param        limit query int    false "Max events to return (default 100, max 500)"
// @Success      200   {array} MonitorEventDTO
// @Router       /monitors/{id}/events [get]
func (h *UptimeHandler) GetMonitorEvents(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > 500 {
				n = 500
			}
			limit = n
		}
	}

	// Two complementary time-range shortcuts:
	//   ?date=YYYY-MM-DD       — whole UTC day (used by the Slack digest deep-link).
	//   ?from=...&to=...       — explicit RFC3339 range (used by IncidentCard expand
	//                             to load events that fell inside an outage window).
	// from+to win over date if both are supplied.
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	var (
		start, end time.Time
		err        error
	)
	if fromStr != "" || toStr != "" {
		if fromStr == "" || toStr == "" {
			http.Error(w, "from and to must be provided together", http.StatusBadRequest)
			return
		}
		start, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			http.Error(w, "Invalid from (expected RFC3339)", http.StatusBadRequest)
			return
		}
		end, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			http.Error(w, "Invalid to (expected RFC3339)", http.StatusBadRequest)
			return
		}
		// Inverted ranges silently return [] from the underlying SQL; reject them
		// explicitly so client-side bugs (e.g. swapped args) fail loudly.
		if !start.Before(end) {
			http.Error(w, "from must be before to", http.StatusBadRequest)
			return
		}
	} else {
		start, end, err = parseDateParam(r.URL.Query().Get("date"))
		if err != nil {
			http.Error(w, "Invalid date (expected YYYY-MM-DD)", http.StatusBadRequest)
			return
		}
	}

	var events []db.MonitorEvent
	if start.IsZero() {
		events, err = h.store.GetMonitorEvents(id, limit)
	} else {
		events, err = h.store.GetMonitorEventsBetween(id, start, end, limit)
	}
	if err != nil {
		http.Error(w, "Failed to fetch events: "+err.Error(), http.StatusInternalServerError)
		return
	}

	out := make([]MonitorEventDTO, 0, len(events))
	for _, e := range events {
		out = append(out, toEventDTO(e))
	}
	writeJSON(w, http.StatusOK, out)
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
						if hasHistory && isUp && (isDegraded || latency > task.GetLatencyThreshold()) {
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
