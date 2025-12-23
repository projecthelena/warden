package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/clusteruptime/clusteruptime/internal/config"
	"github.com/clusteruptime/clusteruptime/internal/db"
)

type contextKey string

const contextKeyUserID contextKey = "userID"

type AuthHandler struct {
	store  *db.Store
	config *config.Config
}

func NewAuthHandler(store *db.Store, cfg *config.Config) *AuthHandler {
	return &AuthHandler{store: store, config: cfg}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	user, err := h.store.Authenticate(req.Username, req.Password)
	if err != nil {
		// Avoid leaking specific error details in prod usually, but specific errors help debugging
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Generate Token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}
	token := hex.EncodeToString(tokenBytes)
	expiresAt := time.Now().Add(30 * 24 * time.Hour)

	if err := h.store.CreateSession(user.ID, token, expiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, "session error")
		return
	}

	// Set Cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Expires:  expiresAt,
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Secure:   h.config.CookieSecure,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "logged in",
		"user": map[string]any{
			"username": user.Username,
			"id":       user.ID,
		},
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("auth_token")
	if err == nil {
		_ = h.store.DeleteSession(c.Value)
	}

	// Clear Cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		HttpOnly: true,
		Path:     "/",
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	// Simple check if middleware passed user
	userID, ok := r.Context().Value(contextKeyUserID).(int64)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.store.GetUser(userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"username": user.Username,
			"id":       user.ID,
			"timezone": user.Timezone,
			// Add placeholder avatar for UI
			"avatar": "https://github.com/shadcn.png",
		},
	})
}

type UpdateUserRequest struct {
	Password        string `json:"password,omitempty"`
	CurrentPassword string `json:"currentPassword,omitempty"`
	Timezone        string `json:"timezone,omitempty"`
}

func (h *AuthHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(contextKeyUserID).(int64)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	// Verify Current Password if changing password
	if req.Password != "" {
		if req.CurrentPassword == "" {
			writeError(w, http.StatusBadRequest, "current password required to change password")
			return
		}
		if err := h.store.VerifyPassword(userID, req.CurrentPassword); err != nil {
			writeError(w, http.StatusUnauthorized, "current password incorrect")
			return
		}
	}

	if err := h.store.UpdateUser(userID, req.Password, req.Timezone); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "settings updated"})
}

// Middleware

func (h *AuthHandler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Check Bearer Token (API Key)
		authHeader := r.Header.Get("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token := authHeader[7:]
			valid, err := h.store.ValidateAPIKey(token)
			if err == nil && valid {
				// Valid API Key. Identify as generic API user or specific if linked.
				// For now, API Key grants full access.
				// We can inject a special Context value to indicate API Key usage.
				ctx := context.WithValue(r.Context(), contextKeyUserID, int64(0)) // 0 or -1 to indicate API User
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// 2. Check Cookie
		c, err := r.Cookie("auth_token")
		if err != nil {
			// No cookie
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// 3. Validate Session
		sess, err := h.store.GetSession(c.Value)
		if err != nil || sess == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// 4. Inject UserID into Context
		ctx := context.WithValue(r.Context(), contextKeyUserID, sess.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
