package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/clusteruptime/clusteruptime/internal/db"
	"github.com/clusteruptime/clusteruptime/internal/uptime"
	"github.com/go-chi/chi/v5"
)

type MaintenanceHandler struct {
	store   *db.Store
	manager *uptime.Manager
}

func NewMaintenanceHandler(store *db.Store, manager *uptime.Manager) *MaintenanceHandler {
	return &MaintenanceHandler{store: store, manager: manager}
}

type MaintenanceResponse struct {
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
		log.Printf("ERROR: Failed to schedule maintenance: %v", err)
		http.Error(w, "Failed to schedule maintenance", http.StatusInternalServerError)
		return
	}

	// Force triggers Manager to refresh active maintenance windows
	go h.manager.Sync()

	response := MaintenanceResponse{
		ID:             maintenance.ID,
		Title:          maintenance.Title,
		Description:    maintenance.Description,
		Type:           maintenance.Type,
		Severity:       maintenance.Severity,
		Status:         maintenance.Status,
		StartTime:      maintenance.StartTime,
		EndTime:        maintenance.EndTime,
		AffectedGroups: req.AffectedGroups,    // Return original array
		CreatedAt:      maintenance.CreatedAt, // Note: CreatedAt is set by DB default, might be zero here if relying on DB trigger. However, Incident struct doesn't have it set on creation. If we query back it would be there. For response now, leaving zero/empty is acceptable or we should set it. `generateIncidentID` implies we control ID.
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}

func (h *MaintenanceHandler) GetMaintenance(w http.ResponseWriter, r *http.Request) {
	// Return all maintenance for now, or maybe only active/future?
	// Using zero time returns all history + active
	allEvents, err := h.store.GetIncidents(time.Time{})
	if err != nil {
		http.Error(w, "Failed to fetch maintenance events", http.StatusInternalServerError)
		return
	}

	var dtos []MaintenanceResponse
	for _, i := range allEvents {
		if i.Type != "maintenance" {
			continue
		}

		var groups []string
		if i.AffectedGroups != "" {
			_ = json.Unmarshal([]byte(i.AffectedGroups), &groups)
		}

		dtos = append(dtos, MaintenanceResponse{
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

func (h *MaintenanceHandler) UpdateMaintenance(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Maintenance ID required", http.StatusBadRequest)
		return
	}

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

	// Ensure UTC
	startTime = startTime.UTC()
	endTime = endTime.UTC()

	affectedGroupsJSON, _ := json.Marshal(req.AffectedGroups)

	// Fetch existing to preserve type/created_at if needed, but we can overwrite most.
	// Actually Store.UpdateIncident overwrites fields. Type should stay 'maintenance'.
	// Severity 'minor' default. created_at is NOT updated in SQL.

	incident := db.Incident{
		ID:             id,
		Title:          req.Title,
		Description:    req.Description,
		Type:           "maintenance",
		Severity:       "minor",
		Status:         req.Status,
		StartTime:      startTime,
		EndTime:        &endTime,
		AffectedGroups: string(affectedGroupsJSON),
	}

	if err := h.store.UpdateIncident(incident); err != nil {
		log.Printf("ERROR: Failed to update maintenance %s: %v", id, err)
		http.Error(w, "Failed to update maintenance", http.StatusInternalServerError)
		return
	}

	// Refresh manager
	go h.manager.Sync()

	response := MaintenanceResponse{
		ID:             incident.ID,
		Title:          incident.Title,
		Description:    incident.Description,
		Type:           incident.Type,
		Severity:       incident.Severity,
		Status:         incident.Status,
		StartTime:      incident.StartTime,
		EndTime:        incident.EndTime,
		AffectedGroups: req.AffectedGroups,
		CreatedAt:      time.Time{}, // Unknown without refetch, but UI probably doesn't need it urgently for update
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

func (h *MaintenanceHandler) DeleteMaintenance(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Maintenance ID required", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteIncident(id); err != nil {
		log.Printf("ERROR: Failed to delete maintenance %s: %v", id, err)
		http.Error(w, "Failed to delete maintenance", http.StatusInternalServerError)
		return
	}

	// Refresh manager
	go h.manager.Sync()

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"success":true}`))
}
