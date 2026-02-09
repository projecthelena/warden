package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/uptime"
	"github.com/go-chi/chi/v5"
)

type CRUDHandler struct {
	store   *db.Store
	manager *uptime.Manager
}

func NewCRUDHandler(store *db.Store, manager *uptime.Manager) *CRUDHandler {
	return &CRUDHandler{store: store, manager: manager}
}

// generateID creates a slug + hash ID from a name
// e.g. "My Group" -> "g-my-group-a1b2c3"
func generateID(name, prefix string) string {
	slug := generateSlug(name, prefix)

	// 2. Generate random suffix (3 bytes = 6 hex chars)
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return slug + "-rnd"
	}
	hash := hex.EncodeToString(b)

	return slug + "-" + hash
}

// generateSlug creates a clean slug ID from a name without hash
// e.g. "My Group" -> "g-my-group"
func generateSlug(name, prefix string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	reg := regexp.MustCompile("[^a-z0-9-]+")
	slug = reg.ReplaceAllString(slug, "")
	return prefix + slug
}

// maxNameLength is the maximum allowed length for names (groups, monitors)
const maxNameLength = 255

// CreateGroup creates a new monitor group.
// @Summary      Create group
// @Tags         groups
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{name=string} true "Group payload"
// @Success      201  {object} db.Group
// @Failure      400  {string} string "Name is required"
// @Failure      409  {object} object{error=string} "Group already exists"
// @Router       /groups [post]
func (h *CRUDHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	// SECURITY: Validate name length
	if len(req.Name) > maxNameLength {
		http.Error(w, "Name too long (max 255 characters)", http.StatusBadRequest)
		return
	}

	id := generateSlug(req.Name, "g-")

	g := db.Group{
		ID:   id,
		Name: req.Name,
	}

	if err := h.store.CreateGroup(g); err != nil {
		// Handle Duplicate ID error
		if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "duplicate key") {
			writeError(w, http.StatusConflict, "Group with this name already exists (ID: "+id+")")
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(g)
}

// DeleteGroup deletes a monitor group by ID.
// @Summary      Delete group
// @Tags         groups
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string true "Group ID"
// @Success      200  "OK"
// @Failure      400  {string} string "ID required"
// @Router       /groups/{id} [delete]
func (h *CRUDHandler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteGroup(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.manager.Sync()
	w.WriteHeader(http.StatusOK)
}

// UpdateGroup renames a monitor group.
// @Summary      Update group
// @Tags         groups
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string true "Group ID"
// @Param        body body object{name=string} true "New name"
// @Success      200  {object} object{name=string}
// @Failure      400  {string} string "Name is required"
// @Router       /groups/{id} [put]
func (h *CRUDHandler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	// SECURITY: Validate name length
	if len(req.Name) > maxNameLength {
		http.Error(w, "Name too long (max 255 characters)", http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateGroup(id, req.Name); err != nil {
		http.Error(w, "Failed to update group", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(req)
}

// CreateMonitor creates a new HTTP monitor.
// @Summary      Create monitor
// @Tags         monitors
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{name=string,url=string,groupId=string,interval=int} true "Monitor payload"
// @Success      201  {object} db.Monitor
// @Failure      400  {string} string "Validation error"
// @Failure      404  {string} string "Group not found"
// @Failure      409  {string} string "Monitor name already exists"
// @Router       /monitors [post]
func (h *CRUDHandler) CreateMonitor(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		URL      string `json:"url"`
		GroupID  string `json:"groupId"`
		Interval int    `json:"interval"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 1. Basic Validation
	if req.Name == "" || req.URL == "" || req.GroupID == "" {
		http.Error(w, "Name, URL, and GroupID are required", http.StatusBadRequest)
		return
	}

	// SECURITY: Validate name length
	if len(req.Name) > maxNameLength {
		http.Error(w, "Name too long (max 255 characters)", http.StatusBadRequest)
		return
	}

	// 2. Validate URL
	parsedURL, err := url.ParseRequestURI(req.URL)
	if err != nil {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	// SECURITY: Only allow http and https protocols to prevent SSRF
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		http.Error(w, "Only HTTP and HTTPS URLs are allowed", http.StatusBadRequest)
		return
	}

	// SECURITY: Validate URL length
	if len(req.URL) > 2048 {
		http.Error(w, "URL too long (max 2048 characters)", http.StatusBadRequest)
		return
	}

	// 3. Validate Interval
	if req.Interval < 10 {
		http.Error(w, "Interval must be at least 10 seconds", http.StatusBadRequest)
		return
	}

	// 4. Validate Group Exists
	groups, err := h.store.GetGroups()
	if err != nil {
		http.Error(w, "System error checking groups", http.StatusInternalServerError)
		return
	}
	groupExists := false
	for _, g := range groups {
		if g.ID == req.GroupID {
			groupExists = true
			break
		}
	}
	if !groupExists {
		http.Error(w, "Selected group does not exist", http.StatusNotFound)
		return
	}

	// 5. Validate Duplicate Name (Simulate unique constraint)
	monitors, err := h.store.GetMonitors()
	if err == nil {
		for _, m := range monitors {
			if strings.EqualFold(m.Name, req.Name) {
				http.Error(w, "A monitor with this name already exists", http.StatusConflict)
				return
			}
		}
	}

	id := generateID(req.Name, "m-")

	m := db.Monitor{
		ID:       id,
		GroupID:  req.GroupID,
		Name:     req.Name,
		URL:      req.URL,
		Active:   true,
		Interval: req.Interval,
	}

	if err := h.store.CreateMonitor(m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Notify Engine to start monitoring this new URL immediately
	h.manager.Sync()

	// Wait for the first ping results (max 5 seconds) to ensure "Wow effect" in UI
	// This ensures that when the frontend fetches the list immediately after this returns,
	// the first check is likely already done.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mon := h.manager.GetMonitor(id)
		if mon != nil && len(mon.GetHistory()) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(m)
}

func (h *CRUDHandler) GetGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.store.GetGroups()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(groups)
}

// UpdateMonitor updates a monitor's name, URL, or interval.
// @Summary      Update monitor
// @Tags         monitors
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string true "Monitor ID"
// @Param        body body object{name=string,url=string,interval=int} true "Fields to update"
// @Success      200  "OK"
// @Failure      400  {string} string "ID required"
// @Router       /monitors/{id} [put]
func (h *CRUDHandler) UpdateMonitor(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	var req struct {
		Name     string `json:"name"`
		URL      string `json:"url"`
		Interval int    `json:"interval"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateMonitor(id, req.Name, req.URL, req.Interval); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.manager.Sync()
	w.WriteHeader(http.StatusOK)
}

// DeleteMonitor removes a monitor and its history.
// @Summary      Delete monitor
// @Tags         monitors
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string true "Monitor ID"
// @Success      200  "OK"
// @Failure      400  {string} string "ID required"
// @Router       /monitors/{id} [delete]
func (h *CRUDHandler) DeleteMonitor(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteMonitor(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.manager.RemoveMonitor(id)
	h.manager.Sync()
	w.WriteHeader(http.StatusOK)
}

// PauseMonitor stops checking a monitor without deleting it.
// @Summary      Pause monitor
// @Tags         monitors
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string true "Monitor ID"
// @Success      200  {object} object{message=string,active=bool}
// @Failure      400  {object} object{error=string} "ID required"
// @Failure      404  {object} object{error=string} "Monitor not found"
// @Router       /monitors/{id}/pause [post]
func (h *CRUDHandler) PauseMonitor(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "ID required")
		return
	}

	if err := h.store.SetMonitorActive(id, false); err != nil {
		if errors.Is(err, db.ErrMonitorNotFound) {
			writeError(w, http.StatusNotFound, "monitor not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to pause monitor")
		return
	}

	h.manager.Sync() // Immediately stop the monitor
	writeJSON(w, http.StatusOK, map[string]any{"message": "monitor paused", "active": false})
}

// ResumeMonitor restarts checking a paused monitor.
// @Summary      Resume monitor
// @Tags         monitors
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string true "Monitor ID"
// @Success      200  {object} object{message=string,active=bool}
// @Failure      400  {object} object{error=string} "ID required"
// @Failure      404  {object} object{error=string} "Monitor not found"
// @Router       /monitors/{id}/resume [post]
func (h *CRUDHandler) ResumeMonitor(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "ID required")
		return
	}

	if err := h.store.SetMonitorActive(id, true); err != nil {
		if errors.Is(err, db.ErrMonitorNotFound) {
			writeError(w, http.StatusNotFound, "monitor not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to resume monitor")
		return
	}

	h.manager.Sync() // Immediately start the monitor
	writeJSON(w, http.StatusOK, map[string]any{"message": "monitor resumed", "active": true})
}
