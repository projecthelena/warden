package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/go-chi/chi/v5"
)

type IncidentHandler struct {
	store *db.Store
}

func NewIncidentHandler(store *db.Store) *IncidentHandler {
	return &IncidentHandler{store: store}
}

func generateIncidentID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "inc-" + time.Now().Format("20060102150405")
	}
	return hex.EncodeToString(b)
}

// IncidentResponseDTO is the API response structure for incidents
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
	CreatedAt      time.Time           `json:"createdAt"`
	Source         string              `json:"source"`
	OutageID       *int64              `json:"outageId,omitempty"`
	Public         bool                `json:"public"`
	Updates        []db.IncidentUpdate `json:"updates,omitempty"`
}

func incidentToDTO(i db.Incident, updates []db.IncidentUpdate) IncidentResponseDTO {
	var groups []string
	if i.AffectedGroups != "" {
		_ = json.Unmarshal([]byte(i.AffectedGroups), &groups)
	}

	source := i.Source
	if source == "" {
		source = "manual"
	}

	return IncidentResponseDTO{
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
		Source:         source,
		OutageID:       i.OutageID,
		Public:         i.Public,
		Updates:        updates,
	}
}

// CreateIncident reports a new manual incident.
// @Summary      Create incident
// @Tags         incidents
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{title=string,description=string,severity=string,status=string,startTime=string,affectedGroups=[]string} true "Incident payload"
// @Success      201  {object} db.Incident
// @Failure      400  {string} string "Invalid request body"
// @Router       /incidents [post]
func (h *IncidentHandler) CreateIncident(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title          string   `json:"title"`
		Description    string   `json:"description"`
		Severity       string   `json:"severity"`
		Status         string   `json:"status"`
		StartTime      string   `json:"startTime"` // Expects ISO8601 string
		AffectedGroups []string `json:"affectedGroups"`
		Public         bool     `json:"public"`
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

	// Ensure UTC storage
	startTime = startTime.UTC()

	affectedGroupsJSON, _ := json.Marshal(req.AffectedGroups)

	incident := db.Incident{
		ID:             generateIncidentID(),
		Title:          req.Title,
		Description:    req.Description,
		Type:           "incident", // Enforce type
		Severity:       req.Severity,
		Status:         req.Status,
		StartTime:      startTime,
		AffectedGroups: string(affectedGroupsJSON),
		Source:         "manual",
		Public:         req.Public,
	}

	if err := h.store.CreateIncident(incident); err != nil {
		log.Printf("ERROR: Failed to create incident: %v", err)
		http.Error(w, "Failed to create incident", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(incidentToDTO(incident, nil))
}

// GetIncidents returns incidents from the last 7 days.
// @Summary      List incidents
// @Tags         incidents
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array} IncidentResponseDTO
// @Failure      500  {string} string "Failed to fetch incidents"
// @Router       /incidents [get]
func (h *IncidentHandler) GetIncidents(w http.ResponseWriter, r *http.Request) {
	since := time.Now().Add(-7 * 24 * time.Hour)
	allEvents, err := h.store.GetIncidents(since)
	if err != nil {
		http.Error(w, "Failed to fetch incidents", http.StatusInternalServerError)
		return
	}

	var dtos []IncidentResponseDTO
	for _, i := range allEvents {
		if i.Type != "incident" {
			continue
		}
		dtos = append(dtos, incidentToDTO(i, nil))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dtos)
}

// GetIncident returns a single incident with its timeline updates.
// @Summary      Get incident with timeline
// @Tags         incidents
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Incident ID"
// @Success      200  {object} IncidentResponseDTO
// @Failure      404  {string} string "Incident not found"
// @Router       /incidents/{id} [get]
func (h *IncidentHandler) GetIncident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	incident, err := h.store.GetIncidentByID(id)
	if err != nil {
		log.Printf("ERROR: Failed to get incident: %v", err)
		http.Error(w, "Failed to get incident", http.StatusInternalServerError)
		return
	}
	if incident == nil {
		http.Error(w, "Incident not found", http.StatusNotFound)
		return
	}

	updates, err := h.store.GetIncidentUpdates(id)
	if err != nil {
		log.Printf("ERROR: Failed to get incident updates: %v", err)
		// Continue without updates
		updates = nil
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(incidentToDTO(*incident, updates))
}

// UpdateIncident updates an existing incident.
// @Summary      Update incident
// @Tags         incidents
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Incident ID"
// @Param        body body object{title=string,description=string,severity=string,status=string,startTime=string,endTime=string,affectedGroups=[]string,public=bool} true "Incident payload"
// @Success      200  {object} IncidentResponseDTO
// @Failure      400  {string} string "Invalid request body"
// @Failure      404  {string} string "Incident not found"
// @Router       /incidents/{id} [put]
func (h *IncidentHandler) UpdateIncident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	existing, err := h.store.GetIncidentByID(id)
	if err != nil {
		log.Printf("ERROR: Failed to get incident: %v", err)
		http.Error(w, "Failed to get incident", http.StatusInternalServerError)
		return
	}
	if existing == nil {
		http.Error(w, "Incident not found", http.StatusNotFound)
		return
	}

	var req struct {
		Title          string   `json:"title"`
		Description    string   `json:"description"`
		Severity       string   `json:"severity"`
		Status         string   `json:"status"`
		StartTime      string   `json:"startTime"`
		EndTime        *string  `json:"endTime"`
		AffectedGroups []string `json:"affectedGroups"`
		Public         bool     `json:"public"`
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

	var endTime *time.Time
	if req.EndTime != nil && *req.EndTime != "" {
		et, err := time.Parse(time.RFC3339, *req.EndTime)
		if err != nil {
			http.Error(w, "Invalid end time format", http.StatusBadRequest)
			return
		}
		et = et.UTC()
		endTime = &et
	}

	affectedGroupsJSON, _ := json.Marshal(req.AffectedGroups)

	incident := db.Incident{
		ID:             id,
		Title:          req.Title,
		Description:    req.Description,
		Type:           existing.Type,
		Severity:       req.Severity,
		Status:         req.Status,
		StartTime:      startTime.UTC(),
		EndTime:        endTime,
		AffectedGroups: string(affectedGroupsJSON),
		Source:         existing.Source,
		OutageID:       existing.OutageID,
		Public:         req.Public,
	}

	if err := h.store.UpdateIncident(incident); err != nil {
		log.Printf("ERROR: Failed to update incident: %v", err)
		http.Error(w, "Failed to update incident", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(incidentToDTO(incident, nil))
}

// DeleteIncident deletes an incident.
// @Summary      Delete incident
// @Tags         incidents
// @Security     BearerAuth
// @Param        id path string true "Incident ID"
// @Success      204
// @Failure      500  {string} string "Failed to delete incident"
// @Router       /incidents/{id} [delete]
func (h *IncidentHandler) DeleteIncident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.store.DeleteIncident(id); err != nil {
		log.Printf("ERROR: Failed to delete incident: %v", err)
		http.Error(w, "Failed to delete incident", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// PromoteOutage creates an incident from an auto-detected outage.
// @Summary      Promote outage to incident
// @Tags         incidents
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Outage ID"
// @Param        body body object{title=string,description=string,severity=string,affectedGroups=[]string} true "Incident details"
// @Success      201  {object} IncidentResponseDTO
// @Failure      400  {string} string "Invalid request body"
// @Failure      404  {string} string "Outage not found"
// @Router       /outages/{id}/promote [post]
func (h *IncidentHandler) PromoteOutage(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	outageID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid outage ID", http.StatusBadRequest)
		return
	}

	outage, err := h.store.GetOutageByID(outageID)
	if err != nil {
		log.Printf("ERROR: Failed to get outage: %v", err)
		http.Error(w, "Failed to get outage", http.StatusInternalServerError)
		return
	}
	if outage == nil {
		http.Error(w, "Outage not found", http.StatusNotFound)
		return
	}

	var req struct {
		Title          string   `json:"title"`
		Description    string   `json:"description"`
		Severity       string   `json:"severity"`
		AffectedGroups []string `json:"affectedGroups"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Use defaults from outage if not provided
	title := req.Title
	if title == "" {
		title = "Service Disruption: " + outage.MonitorName
	}
	description := req.Description
	if description == "" {
		description = outage.Summary
	}
	severity := req.Severity
	if severity == "" {
		severity = "critical"
	}
	affectedGroups := req.AffectedGroups
	if len(affectedGroups) == 0 {
		affectedGroups = []string{outage.GroupID}
	}

	affectedGroupsJSON, _ := json.Marshal(affectedGroups)

	// Determine initial status based on whether outage is resolved
	status := "investigating"
	var endTime *time.Time
	if outage.EndTime != nil {
		status = "resolved"
		endTime = outage.EndTime
	}

	incident := db.Incident{
		ID:             generateIncidentID(),
		Title:          title,
		Description:    description,
		Type:           "incident",
		Severity:       severity,
		Status:         status,
		StartTime:      outage.StartTime,
		EndTime:        endTime,
		AffectedGroups: string(affectedGroupsJSON),
		Source:         "auto",
		OutageID:       &outageID,
		Public:         false, // Requires explicit approval to make public
	}

	if err := h.store.CreateIncident(incident); err != nil {
		log.Printf("ERROR: Failed to create incident from outage: %v", err)
		http.Error(w, "Failed to create incident", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(incidentToDTO(incident, nil))
}

// SetVisibility toggles the public visibility of an incident.
// @Summary      Set incident visibility
// @Tags         incidents
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Incident ID"
// @Param        body body object{public=bool} true "Visibility setting"
// @Success      200  {object} object{message=string,public=bool}
// @Failure      400  {string} string "Invalid request body"
// @Failure      404  {string} string "Incident not found"
// @Router       /incidents/{id}/visibility [patch]
func (h *IncidentHandler) SetVisibility(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	incident, err := h.store.GetIncidentByID(id)
	if err != nil {
		log.Printf("ERROR: Failed to get incident: %v", err)
		http.Error(w, "Failed to get incident", http.StatusInternalServerError)
		return
	}
	if incident == nil {
		http.Error(w, "Incident not found", http.StatusNotFound)
		return
	}

	var req struct {
		Public bool `json:"public"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.store.SetIncidentPublic(id, req.Public); err != nil {
		log.Printf("ERROR: Failed to set incident visibility: %v", err)
		http.Error(w, "Failed to set visibility", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Visibility updated",
		"public":  req.Public,
	})
}

// AddUpdate adds a status update to an incident timeline.
// @Summary      Add incident update
// @Tags         incidents
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Incident ID"
// @Param        body body object{status=string,message=string} true "Update details"
// @Success      201  {object} db.IncidentUpdate
// @Failure      400  {string} string "Invalid request body"
// @Failure      404  {string} string "Incident not found"
// @Router       /incidents/{id}/updates [post]
func (h *IncidentHandler) AddUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	incident, err := h.store.GetIncidentByID(id)
	if err != nil {
		log.Printf("ERROR: Failed to get incident: %v", err)
		http.Error(w, "Failed to get incident", http.StatusInternalServerError)
		return
	}
	if incident == nil {
		http.Error(w, "Incident not found", http.StatusNotFound)
		return
	}

	var req struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Status == "" || req.Message == "" {
		http.Error(w, "Status and message are required", http.StatusBadRequest)
		return
	}

	if err := h.store.CreateIncidentUpdate(id, req.Status, req.Message); err != nil {
		log.Printf("ERROR: Failed to create incident update: %v", err)
		http.Error(w, "Failed to create update", http.StatusInternalServerError)
		return
	}

	// Also update the incident's status if it changed
	if req.Status != incident.Status {
		incident.Status = req.Status
		// If resolving, set end time
		if req.Status == "resolved" || req.Status == "completed" {
			now := time.Now()
			incident.EndTime = &now
		}
		_ = h.store.UpdateIncident(*incident)
	}

	// Return the latest updates
	updates, _ := h.store.GetIncidentUpdates(id)
	var latestUpdate db.IncidentUpdate
	if len(updates) > 0 {
		latestUpdate = updates[len(updates)-1]
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(latestUpdate)
}

// GetUpdates returns all updates for an incident.
// @Summary      Get incident updates
// @Tags         incidents
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Incident ID"
// @Success      200  {array} db.IncidentUpdate
// @Failure      404  {string} string "Incident not found"
// @Router       /incidents/{id}/updates [get]
func (h *IncidentHandler) GetUpdates(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	incident, err := h.store.GetIncidentByID(id)
	if err != nil {
		log.Printf("ERROR: Failed to get incident: %v", err)
		http.Error(w, "Failed to get incident", http.StatusInternalServerError)
		return
	}
	if incident == nil {
		http.Error(w, "Incident not found", http.StatusNotFound)
		return
	}

	updates, err := h.store.GetIncidentUpdates(id)
	if err != nil {
		log.Printf("ERROR: Failed to get incident updates: %v", err)
		http.Error(w, "Failed to get updates", http.StatusInternalServerError)
		return
	}

	if updates == nil {
		updates = []db.IncidentUpdate{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updates)
}
