package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/clusteruptime/clusteruptime/internal/db"
	"github.com/clusteruptime/clusteruptime/internal/uptime"
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

func (h *CRUDHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
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
	json.NewEncoder(w).Encode(g)
}

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
	w.WriteHeader(http.StatusOK)
}

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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateGroup(id, req.Name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(req)
}

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

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(m)
}

func (h *CRUDHandler) GetGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.store.GetGroups()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(groups)
}

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
	h.manager.Sync()
	w.WriteHeader(http.StatusOK)
}
