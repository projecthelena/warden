package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/projecthelena/warden/internal/db"
)

type UserHandler struct {
	store *db.Store
}

func NewUserHandler(store *db.Store) *UserHandler {
	return &UserHandler{store: store}
}

// CreateUser creates a new user (admin only).
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleAdmin) {
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"` // #nosec G117 -- input-only DTO, never serialized in responses
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	if req.Role == "" {
		req.Role = RoleViewer
	}
	if !ValidRole(req.Role) {
		writeError(w, http.StatusBadRequest, "invalid role")
		return
	}

	if err := h.store.CreateUser(req.Username, req.Password, "UTC", req.Role); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			writeError(w, http.StatusConflict, "username already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"message": "user created"})
}

// ListUsers returns all users (admin only).
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleAdmin) {
		return
	}

	users, err := h.store.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	type UserDTO struct {
		ID          int64  `json:"id"`
		Username    string `json:"username"`
		Role        string `json:"role"`
		Email       string `json:"email,omitempty"`
		SSOProvider string `json:"ssoProvider,omitempty"`
		AvatarURL   string `json:"avatar,omitempty"`
		DisplayName string `json:"displayName,omitempty"`
		CreatedAt   string `json:"createdAt"`
	}

	dtos := make([]UserDTO, 0, len(users))
	for _, u := range users {
		dtos = append(dtos, UserDTO{
			ID:          u.ID,
			Username:    u.Username,
			Role:        u.Role,
			Email:       u.Email,
			SSOProvider: u.SSOProvider,
			AvatarURL:   u.AvatarURL,
			DisplayName: u.DisplayName,
			CreatedAt:   u.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"users": dtos})
}

// UpdateUserRole changes a user's role (admin only).
func (h *UserHandler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleAdmin) {
		return
	}

	idStr := chi.URLParam(r, "id")
	targetID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	// Cannot change own role
	currentUserID, _ := r.Context().Value(contextKeyUserID).(int64)
	if targetID == currentUserID {
		writeError(w, http.StatusBadRequest, "cannot change your own role")
		return
	}

	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if !ValidRole(req.Role) {
		writeError(w, http.StatusBadRequest, "invalid role")
		return
	}

	// If demoting from admin, ensure at least one admin remains
	targetUser, err := h.store.GetUser(targetID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if targetUser.Role == RoleAdmin && req.Role != RoleAdmin {
		count, err := h.store.CountAdmins()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check admin count")
			return
		}
		if count <= 1 {
			writeError(w, http.StatusBadRequest, "cannot remove the last admin")
			return
		}
	}

	if err := h.store.UpdateUserRole(targetID, req.Role); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update role")
		return
	}

	// Clean up status page assignments when role changes away from status_viewer
	if targetUser.Role == RoleStatusViewer && req.Role != RoleStatusViewer {
		_ = h.store.SetUserStatusPages(targetID, []int64{})
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "role updated"})
}

// DeleteUser removes a user (admin only).
func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleAdmin) {
		return
	}

	idStr := chi.URLParam(r, "id")
	targetID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	// Cannot delete self
	currentUserID, _ := r.Context().Value(contextKeyUserID).(int64)
	if targetID == currentUserID {
		writeError(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}

	// Cannot delete the last admin
	targetUser, err := h.store.GetUser(targetID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if targetUser.Role == RoleAdmin {
		count, err := h.store.CountAdmins()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check admin count")
			return
		}
		if count <= 1 {
			writeError(w, http.StatusBadRequest, "cannot delete the last admin")
			return
		}
	}

	if err := h.store.DeleteUser(targetID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "user deleted"})
}

// GetUserStatusPages returns the status page IDs assigned to a user (admin only).
func (h *UserHandler) GetUserStatusPages(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleAdmin) {
		return
	}

	idStr := chi.URLParam(r, "id")
	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	pageIDs, err := h.store.GetUserStatusPages(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get status pages")
		return
	}
	if pageIDs == nil {
		pageIDs = []int64{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"statusPageIds": pageIDs})
}

// SetUserStatusPages replaces the status page assignments for a user (admin only).
func (h *UserHandler) SetUserStatusPages(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, RoleAdmin) {
		return
	}

	idStr := chi.URLParam(r, "id")
	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var req struct {
		StatusPageIDs []int64 `json:"statusPageIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if err := h.store.SetUserStatusPages(userID, req.StatusPageIDs); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set status pages")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "status pages updated"})
}
