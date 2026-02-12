package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

type SetupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"` // #nosec G117 -- input-only DTO, never serialized in responses
	Timezone string `json:"timezone"`
}

func (h *Router) CheckSetup(w http.ResponseWriter, r *http.Request) {
	hasUsers, err := h.store.HasUsers()
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	val, _ := h.store.GetSetting("setup_completed")

	_ = json.NewEncoder(w).Encode(map[string]bool{
		"isSetup": hasUsers || val == "true",
	})
}

func (h *Router) PerformSetup(w http.ResponseWriter, r *http.Request) {
	clientIP := extractIP(r)

	// SECURITY: Atomic check for setup completion to prevent race conditions
	// This prevents multiple concurrent requests from creating multiple admin users
	isComplete, err := h.store.IsSetupComplete()
	if err != nil {
		log.Printf("AUDIT: [SETUP] Database error checking setup status from IP %s: %v", sanitizeLog(clientIP), err) // #nosec G706 -- sanitized
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if isComplete {
		log.Printf("AUDIT: [SECURITY] Setup attempt from IP %s denied - setup already completed", sanitizeLog(clientIP)) // #nosec G706 -- sanitized
		http.Error(w, "Setup already completed", http.StatusForbidden)
		return
	}

	// ADMIN_SECRET validation:
	// - If no users exist (first setup), allow without secret (browser-based setup)
	// - If users exist, require secret for programmatic setup (prevents unauthorized reset+setup)
	// This allows first-time browser setup while still protecting against unauthorized
	// programmatic setup after the first user exists.
	hasUsers, _ := h.store.HasUsers()
	if h.config.AdminSecret != "" && hasUsers {
		secretHeader := r.Header.Get("X-Admin-Secret")
		authHeader := r.Header.Get("Authorization")
		bearerSecret := ""
		if strings.HasPrefix(authHeader, "Bearer ") {
			bearerSecret = strings.TrimPrefix(authHeader, "Bearer ")
		}

		// Use constant-time comparison to prevent timing attacks
		headerMatch := subtle.ConstantTimeCompare([]byte(secretHeader), []byte(h.config.AdminSecret)) == 1
		bearerMatch := bearerSecret != "" && subtle.ConstantTimeCompare([]byte(bearerSecret), []byte(h.config.AdminSecret)) == 1

		if !headerMatch && !bearerMatch {
			log.Printf("AUDIT: [SECURITY] Setup attempt from IP %s denied - invalid admin secret (users exist)", sanitizeLog(clientIP)) // #nosec G706 -- sanitized
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
	}

	var req SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validation
	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password required", http.StatusBadRequest)
		return
	}

	// Username Validation
	req.Username = strings.TrimSpace(req.Username)
	if len(req.Username) > 32 {
		http.Error(w, "Username too long (max 32 chars)", http.StatusBadRequest)
		return
	}
	validUsername := regexp.MustCompile(`^[a-z0-9._-]+$`)
	if !validUsername.MatchString(req.Username) {
		http.Error(w, "Username invalid: must be lowercase, alphanumeric, dots, underscores, or dashes only", http.StatusBadRequest)
		return
	}

	// Password validation: 8+ chars, at least one number, at least one special character
	if len(req.Password) < 8 {
		http.Error(w, "Password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	hasNumber := false
	for _, c := range req.Password {
		if c >= '0' && c <= '9' {
			hasNumber = true
			break
		}
	}
	if !hasNumber {
		http.Error(w, "Password must contain at least one number", http.StatusBadRequest)
		return
	}
	hasSpecial := false
	for _, c := range req.Password {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			hasSpecial = true
			break
		}
	}
	if !hasSpecial {
		http.Error(w, "Password must contain at least one special character", http.StatusBadRequest)
		return
	}

	if req.Timezone == "" {
		req.Timezone = "UTC"
	}

	// Create User
	if err := h.store.CreateUser(req.Username, req.Password, req.Timezone); err != nil {
		log.Printf("AUDIT: [SETUP] Failed to create user from IP %s: %v", sanitizeLog(clientIP), err) // #nosec G706 -- sanitized
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}
	log.Printf("AUDIT: [SETUP] Admin user '%s' created from IP %s", sanitizeLog(req.Username), sanitizeLog(clientIP)) // #nosec G706 -- sanitized

	// Always create default monitors (no toggle needed - gives immediate value)
	defaults := []struct{ Name, URL string }{
		{"Google", "https://google.com"},
		{"GitHub", "https://github.com"},
		{"Cloudflare DNS", "https://1.1.1.1"},
	}

	for i, d := range defaults {
		id := fmt.Sprintf("m-default-%d", i)
		if err := h.store.CreateMonitor(db.Monitor{
			ID:       id,
			GroupID:  "g-default",
			Name:     d.Name,
			URL:      d.URL,
			Active:   true,
			Interval: 60,
		}); err != nil {
			// Best effort - ignore errors
			_ = err
		}
	}

	// Mark as completed
	_ = h.store.SetSetting("setup_completed", "true")

	// Trigger immediate check for new monitors
	h.manager.Sync()

	// Wait for all default monitors to get their first ping (max 5s)
	// This ensures the dashboard shows live data immediately after setup
	monitorIDs := []string{"m-default-0", "m-default-1", "m-default-2"}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		allReady := true
		for _, id := range monitorIDs {
			mon := h.manager.GetMonitor(id)
			if mon == nil || len(mon.GetHistory()) == 0 {
				allReady = false
				break
			}
		}
		if allReady {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Auto-login: Create session and set cookie
	// First, authenticate to get user ID
	user, err := h.store.Authenticate(req.Username, req.Password)
	if err != nil {
		// User was created but auth failed - shouldn't happen but handle gracefully
		log.Printf("AUDIT: [SETUP] Auto-login failed for '%s': %v", req.Username, err)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
		})
		return
	}

	// Generate session token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		log.Printf("AUDIT: [SETUP] Failed to generate session token: %v", err)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
		})
		return
	}
	token := hex.EncodeToString(tokenBytes)
	expiresAt := time.Now().Add(30 * 24 * time.Hour)

	if err := h.store.CreateSession(user.ID, token, expiresAt); err != nil {
		log.Printf("AUDIT: [SETUP] Failed to create session: %v", err)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
		})
		return
	}

	// Set auth cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Expires:  expiresAt,
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Secure:   h.config.CookieSecure,
	})

	log.Printf("AUDIT: [SETUP] Auto-login successful for '%s' from IP %s", sanitizeLog(req.Username), sanitizeLog(clientIP)) // #nosec G706 -- sanitized

	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"user": map[string]any{
			"username": user.Username,
			"id":       user.ID,
		},
	})
}
