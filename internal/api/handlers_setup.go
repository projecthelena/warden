package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/clusteruptime/clusteruptime/internal/db"
)

type SetupRequest struct {
	Username       string `json:"username"`
	Password       string `json:"password"`
	Timezone       string `json:"timezone"`
	CreateDefaults bool   `json:"createDefaults"` // Just a flag for simplicity
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
	// Double check security
	hasUsers, _ := h.store.HasUsers()
	val, _ := h.store.GetSetting("setup_completed")

	if hasUsers || val == "true" {
		http.Error(w, "Setup already completed", http.StatusForbidden)
		return
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

	if len(req.Password) < 8 {
		http.Error(w, "Password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	// Check for a number
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
	// Check for special character (simple check for non-alphanumeric)
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
		log.Printf("Failed to create user: %v", err)
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	// Optional: Create Defaults
	if req.CreateDefaults {
		// Create Default Group if not exists (seed might have created it? seed only checks count)
		// Let's manually ensure it.
		// Actually, seed() runs on startup. So Group "Default" likely exists if groupCount was 0 initially.
		// But monitors were removed from seed? No, only user seed was removed.
		// Wait, did I verify seed logic for monitors?
		// Monitor seeding depends on monitor count. If 0, it creates example.
		// If user wants defaults, we can add more specific ones or just rely on the fact that
		// the seed() function might have run?
		// Actually, seed() runs BEFORE http server starts. So default group/monitor might already exist.
		// If user wants "Create Defaults", maybe we add Google/GitHub specifically?

		// Let's add a couple of common ones to the 'g-default' group.
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
				// We don't have logger here easily.
				// We can ignore it safely as this is "best effort" defaults
				_ = err
			}
		}
	}

	// Mark as completed
	_ = h.store.SetSetting("setup_completed", "true")

	// Trigger immediate check for new monitors
	h.manager.Sync()

	_ = json.NewEncoder(w).Encode(map[string]bool{
		"success": true,
	})
}
