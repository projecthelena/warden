package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/projecthelena/warden/internal/db"
	"github.com/go-chi/chi/v5"
)

type APIKeyHandler struct {
	store *db.Store
}

func NewAPIKeyHandler(store *db.Store) *APIKeyHandler {
	return &APIKeyHandler{store: store}
}

func (h *APIKeyHandler) ListKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.store.ListAPIKeys()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list keys")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
}

func (h *APIKeyHandler) CreateKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	rawKey, err := h.store.CreateAPIKey(req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create key")
		return
	}

	// Return the raw key ONLY ONCE
	writeJSON(w, http.StatusOK, map[string]string{
		"key":     rawKey,
		"message": "Key created. Save it now, it will not be shown again.",
	})
}

func (h *APIKeyHandler) DeleteKey(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.store.DeleteAPIKey(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete key")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}
