package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/clusteruptime/clusteruptime/internal/db"
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

func (h *IncidentHandler) CreateIncident(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title          string   `json:"title"`
		Description    string   `json:"description"`
		Severity       string   `json:"severity"`
		Status         string   `json:"status"`
		StartTime      string   `json:"startTime"` // Expects ISO8601 string
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
	}

	if err := h.store.CreateIncident(incident); err != nil {
		log.Printf("ERROR: Failed to create incident: %v", err)
		http.Error(w, "Failed to create incident", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(incident)
}

func (h *IncidentHandler) GetIncidents(w http.ResponseWriter, r *http.Request) {
	since := time.Now().Add(-7 * 24 * time.Hour)
	allEvents, err := h.store.GetIncidents(since)
	if err != nil {
		http.Error(w, "Failed to fetch incidents", http.StatusInternalServerError)
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
		if i.Type != "incident" {
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
