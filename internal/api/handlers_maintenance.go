package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/clusteruptime/clusteruptime/internal/db"
)

type MaintenanceHandler struct {
	store *db.Store
}

func NewMaintenanceHandler(store *db.Store) *MaintenanceHandler {
	return &MaintenanceHandler{store: store}
}

func (h *MaintenanceHandler) CreateMaintenance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title          string   `json:"title"`
		Description    string   `json:"description"`
		Status         string   `json:"status"`
		StartTime      string   `json:"startTime"`
		EndTime        string   `json:"endTime"`
		AffectedGroups []string `json:"affectedGroups"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		http.Error(w, "Invalid start time format", http.StatusBadRequest)
		return
	}

	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		http.Error(w, "Invalid end time format", http.StatusBadRequest)
		return
	}

	// Ensure UTC storage
	startTime = startTime.UTC()
	endTime = endTime.UTC()

	affectedGroupsJSON, _ := json.Marshal(req.AffectedGroups)

	maintenance := db.Incident{
		ID:             generateIncidentID(), // Still in package api, so accessible if in same package
		Title:          req.Title,
		Description:    req.Description,
		Type:           "maintenance", // Enforce type
		Severity:       "minor",       // Default for maintenance
		Status:         req.Status,
		StartTime:      startTime,
		EndTime:        &endTime,
		AffectedGroups: string(affectedGroupsJSON),
	}

	if err := h.store.CreateIncident(maintenance); err != nil {
		http.Error(w, "Failed to schedule maintenance: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(maintenance)
}

func (h *MaintenanceHandler) GetMaintenance(w http.ResponseWriter, r *http.Request) {
	// Return all maintenance for now, or maybe only active/future?
	// Using zero time returns all history + active
	allEvents, err := h.store.GetIncidents(time.Time{})
	if err != nil {
		http.Error(w, "Failed to fetch maintenance events", http.StatusInternalServerError)
		return
	}

	type IncidentDTO struct {
		ID             string     `json:"id"`
		Title          string     `json:"title"`
		Description    string     `json:"description"`
		Type           string     `json:"type"`
		Severity       string     `json:"severity"`
		Status         string     `json:"status"`
		StartTime      time.Time  `json:"startTime"`
		EndTime        *time.Time `json:"endTime,omitempty"`
		AffectedGroups []string   `json:"affectedGroups"`
		CreatedAt      time.Time  `json:"createdAt"`
	}

	var dtos []IncidentDTO
	for _, i := range allEvents {
		if i.Type != "maintenance" {
			continue
		}

		var groups []string
		if i.AffectedGroups != "" {
			_ = json.Unmarshal([]byte(i.AffectedGroups), &groups)
		}

		dtos = append(dtos, IncidentDTO{
			ID:             i.ID,
			Title:          i.Title,
			Description:    i.Description,
			Type:           i.Type,
			Severity:       i.Severity,
			Status:         i.Status,
			StartTime:      i.StartTime,
			EndTime:        i.EndTime,
			AffectedGroups: groups,
			CreatedAt:      i.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dtos)
}
