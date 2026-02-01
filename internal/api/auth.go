package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/clusteruptime/clusteruptime/internal/config"
	"github.com/clusteruptime/clusteruptime/internal/db"
)

type contextKey string

const contextKeyUserID contextKey = "userID"

// APIKeyUserID is used to identify requests authenticated via API key
// SECURITY: Use -1 to distinguish from real user IDs (which are positive)
// This prevents authorization bypass if handlers assume userID > 0 means valid user
const APIKeyUserID int64 = -1

type AuthHandler struct {
	store        *db.Store
	config       *config.Config
	loginLimiter *LoginRateLimiter
}

func NewAuthHandler(store *db.Store, cfg *config.Config, loginLimiter *LoginRateLimiter) *AuthHandler {
	return &AuthHandler{store: store, config: cfg, loginLimiter: loginLimiter}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	// Extract client IP for rate limiting
	ip := extractIP(r)

	// Check if IP is currently blocked due to too many failed attempts
	if h.loginLimiter != nil && !h.loginLimiter.Allow(ip) {
		blockDuration := h.loginLimiter.BlockDuration(ip)
		w.Header().Set("Retry-After", fmt.Sprintf("%d", int(blockDuration.Seconds())+1))
		writeError(w, http.StatusTooManyRequests, "too many failed login attempts, please try again later")
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	// Check if username is currently blocked (distributed brute force protection)
	if h.loginLimiter != nil && !h.loginLimiter.AllowUsername(req.Username) {
		blockDuration := h.loginLimiter.UsernameBlockDuration(req.Username)
		w.Header().Set("Retry-After", fmt.Sprintf("%d", int(blockDuration.Seconds())+1))
		writeError(w, http.StatusTooManyRequests, "too many failed login attempts for this account, please try again later")
		return
	}

	user, err := h.store.Authenticate(req.Username, req.Password)
	if err != nil {
		// AUDIT: Log failed authentication attempt (username only, never password)
		log.Printf("AUDIT: [AUTH] Failed login attempt for user '%s' from IP %s", req.Username, ip)

		// Record failed attempt for rate limiting (both IP and username)
		if h.loginLimiter != nil {
			h.loginLimiter.RecordFailure(ip)
			h.loginLimiter.RecordUsernameFailure(req.Username)
		}
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// AUDIT: Log successful authentication
	log.Printf("AUDIT: [AUTH] Successful login for user '%s' (ID: %d) from IP %s", user.Username, user.ID, ip)

	// Clear rate limit on successful login (both IP and username)
	if h.loginLimiter != nil {
		h.loginLimiter.RecordSuccess(ip)
		h.loginLimiter.RecordUsernameSuccess(req.Username)
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

	// SECURITY: API keys cannot access user-specific endpoints
	if userID == APIKeyUserID {
		writeError(w, http.StatusForbidden, "API keys cannot access user profile")
		return
	}

	user, err := h.store.GetUser(userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	// Use SSO avatar if available, otherwise use a generated one
	avatar := user.AvatarURL
	displayName := user.DisplayName
	if displayName == "" {
		displayName = user.Username // Fallback to username for non-SSO users
	}
	if avatar == "" {
		// Generate a UI Avatars URL as fallback
		// SECURITY: URL-encode displayName to prevent XSS
		avatar = "https://ui-avatars.com/api/?name=" + url.QueryEscape(displayName) + "&background=random"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"username":    user.Username,
			"id":          user.ID,
			"timezone":    user.Timezone,
			"email":       user.Email,
			"ssoProvider": user.SSOProvider,
			"avatar":      avatar,
			"displayName": displayName,
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

	// SECURITY: API keys cannot modify user settings
	if userID == APIKeyUserID {
		writeError(w, http.StatusForbidden, "API keys cannot modify user settings")
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

	// If password was changed, invalidate all other sessions for security
	if req.Password != "" {
		// AUDIT: Log password change
		clientIP := extractIP(r)
		log.Printf("AUDIT: [AUTH] Password changed for user ID %d from IP %s - invalidating other sessions", userID, clientIP)

		// Get current session token to preserve it
		currentToken := ""
		if c, err := r.Cookie("auth_token"); err == nil {
			currentToken = c.Value
		}
		// Delete all other sessions for this user
		_ = h.store.DeleteUserSessions(userID, currentToken)
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
				// Valid API Key - use special negative ID to distinguish from real users
				// SECURITY: APIKeyUserID (-1) prevents confusion with real user IDs
				ctx := context.WithValue(r.Context(), contextKeyUserID, APIKeyUserID)
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
