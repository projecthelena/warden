package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/clusteruptime/clusteruptime/internal/db"
	"github.com/clusteruptime/clusteruptime/internal/uptime"
)

type EventHandler struct {
	store   *db.Store
	manager *uptime.Manager
}

func NewEventHandler(store *db.Store, manager *uptime.Manager) *EventHandler {
	return &EventHandler{store: store, manager: manager}
}

type IncidentDTO struct {
	ID          string     `json:"id"`
	MonitorID   string     `json:"monitorId"`
	MonitorName string     `json:"monitorName"`
	GroupName   string     `json:"groupName"`
	GroupID     string     `json:"groupId"`
	Type        string     `json:"type"` // down, degraded
	Message     string     `json:"message"`
	StartedAt   time.Time  `json:"startedAt"`
	ResolvedAt  *time.Time `json:"resolvedAt"` // Null if active
	Duration    string     `json:"duration"`
}

func (h *EventHandler) GetSystemEvents(w http.ResponseWriter, r *http.Request) {
	activeOutages, err := h.store.GetActiveOutages()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch active outages")
		return
	}

	since := time.Now().Add(-7 * 24 * time.Hour)
	resolvedOutages, err := h.store.GetResolvedOutages(since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch history")
		return
	}

	var active []IncidentDTO
	for _, o := range activeOutages {
		active = append(active, IncidentDTO{
			ID:          fmt.Sprintf("%d", o.ID),
			MonitorID:   o.MonitorID,
			MonitorName: o.MonitorName,
			GroupName:   o.GroupName,
			GroupID:     o.GroupID,
			Type:        o.Type,
			Message:     o.Summary,
			StartedAt:   o.StartTime,
			Duration:    formatDuration(time.Since(o.StartTime)),
		})
	}

	var history []IncidentDTO
	for _, o := range resolvedOutages {
		dur := "0m"
		if o.EndTime != nil {
			dur = formatDuration(o.EndTime.Sub(o.StartTime))
		}
		history = append(history, IncidentDTO{
			ID:          fmt.Sprintf("%d", o.ID),
			MonitorID:   o.MonitorID,
			MonitorName: o.MonitorName,
			GroupName:   o.GroupName,
			GroupID:     o.GroupID,
			Type:        o.Type,
			Message:     o.Summary,
			StartedAt:   o.StartTime,
			ResolvedAt:  o.EndTime,
			Duration:    dur,
		})
	}

	// Returns empty arrays if nil
	if active == nil {
		active = []IncidentDTO{}
	}
	if history == nil {
		history = []IncidentDTO{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"active":  active,
		"history": history,
	})
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
