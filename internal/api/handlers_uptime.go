package api

import (
	"encoding/json"
	"net/http"

	"github.com/clusteruptime/clusteruptime/internal/db"
	"github.com/clusteruptime/clusteruptime/internal/uptime"
)

type UptimeHandler struct {
	manager *uptime.Manager
	store   *db.Store
}

func NewUptimeHandler(manager *uptime.Manager, store *db.Store) *UptimeHandler {
	return &UptimeHandler{manager: manager, store: store}
}

// Response Structures matching Frontend Store
type MonitorDTO struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	URL       string   `json:"url"`
	Status    string   `json:"status"`
	Latency   int64    `json:"latency"`
	History   []string `json:"history"`
	LastCheck string   `json:"lastCheck"`
}

type GroupDTO struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Monitors []MonitorDTO `json:"monitors"`
}

type UptimeResponse struct {
	Groups []GroupDTO `json:"groups"`
}

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

	for _, g := range groups {
		monitorDTOs := []MonitorDTO{} // Ensure initialized as empty slice, not nil

		for _, meta := range groupMap[g.ID] {
			// Get Live Status from Manager
			task := h.manager.GetMonitor(meta.ID)

			statusStr := "down" // Default if not running
			latency := int64(0)
			lastCheck := "Never"
			var historyStr []string

			if task != nil {
				// It is running
				history := task.GetHistory()

				if len(history) > 0 {
					last := history[len(history)-1]
					if last.IsUp {
						statusStr = "up"
						if last.Latency > 500 {
							statusStr = "degraded"
						}
					}
					latency = last.Latency
					lastCheck = last.Timestamp.Format("15:04:05")

					for _, h := range history {
						s := "down"
						if h.IsUp {
							s = "up"
							if h.Latency > 500 {
								s = "degraded"
							}
						}
						historyStr = append(historyStr, s)
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
				Latency:   latency,
				History:   historyStr,
				LastCheck: lastCheck,
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
	json.NewEncoder(w).Encode(resp)
}
