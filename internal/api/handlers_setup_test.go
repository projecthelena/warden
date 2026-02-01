package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/clusteruptime/clusteruptime/internal/config"
	"github.com/clusteruptime/clusteruptime/internal/db"
	"github.com/clusteruptime/clusteruptime/internal/uptime"
	"github.com/go-chi/chi/v5"
)

const testAdminSecret = "test-admin-secret-12345"

func TestPerformSetup_Validation(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	// No AdminSecret configured - setup should work without it
	cfg := &config.Config{}
	r := &Router{Mux: chi.NewRouter(), manager: m, store: s, config: cfg}

	tests := []struct {
		name       string
		username   string
		password   string
		wantStatus int
	}{
		{
			name:       "Valid Setup",
			username:   "valid-user",
			password:   "Password1!", // 8+ chars, number, special char
			wantStatus: http.StatusOK,
		},
		{
			name:       "Username Too Long",
			username:   strings.Repeat("a", 33),
			password:   "Password1!",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Valid with Dot and Dash",
			username:   "valid.user-name",
			password:   "MyPass123!",
			wantStatus: http.StatusOK,
		},
		{
			name:       "Invalid characters (Space)",
			username:   "User Name",
			password:   "Password1!",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Invalid characters (Uppercase)",
			username:   "ValidUser",
			password:   "Password1!",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Password too short",
			username:   "valid",
			password:   "Pass1!",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Password missing number",
			username:   "valid",
			password:   "Password!",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Password missing special char",
			username:   "valid",
			password:   "Password1",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := s.Reset(); err != nil {
				t.Fatalf("failed to reset store: %v", err)
			}
			body, _ := json.Marshal(map[string]interface{}{
				"username": tt.username,
				"password": tt.password,
				"timezone": "UTC",
			})
			req := httptest.NewRequest("POST", "/api/setup", bytes.NewBuffer(body))
			// No X-Admin-Secret header needed anymore
			w := httptest.NewRecorder()

			r.PerformSetup(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Test %s: Expected status %d, got %d. Body: %s", tt.name, tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestPerformSetup_WithAdminSecret(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	// AdminSecret IS configured - setup requires it
	cfg := &config.Config{AdminSecret: testAdminSecret}
	r := &Router{Mux: chi.NewRouter(), manager: m, store: s, config: cfg}

	t.Run("Requires admin secret when configured", func(t *testing.T) {
		if err := s.Reset(); err != nil {
			t.Fatalf("failed to reset store: %v", err)
		}
		body, _ := json.Marshal(map[string]interface{}{
			"username": "admin",
			"password": "Password1!",
			"timezone": "UTC",
		})
		// No secret header - should fail
		req := httptest.NewRequest("POST", "/api/setup", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		r.PerformSetup(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 when admin secret missing, got %d", w.Code)
		}
	})

	t.Run("Works with correct admin secret", func(t *testing.T) {
		if err := s.Reset(); err != nil {
			t.Fatalf("failed to reset store: %v", err)
		}
		body, _ := json.Marshal(map[string]interface{}{
			"username": "admin",
			"password": "Password1!",
			"timezone": "UTC",
		})
		req := httptest.NewRequest("POST", "/api/setup", bytes.NewBuffer(body))
		req.Header.Set("X-Admin-Secret", testAdminSecret)
		w := httptest.NewRecorder()

		r.PerformSetup(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 with correct admin secret, got %d. Body: %s", w.Code, w.Body.String())
		}
	})
}

func TestPerformSetup_AutoLogin(t *testing.T) {
	s, _ := db.NewStore(db.NewTestConfig())
	m := uptime.NewManager(s)
	cfg := &config.Config{}
	r := &Router{Mux: chi.NewRouter(), manager: m, store: s, config: cfg}

	t.Run("Sets auth cookie on successful setup", func(t *testing.T) {
		if err := s.Reset(); err != nil {
			t.Fatalf("failed to reset store: %v", err)
		}
		body, _ := json.Marshal(map[string]interface{}{
			"username": "admin",
			"password": "Password1!",
			"timezone": "UTC",
		})
		req := httptest.NewRequest("POST", "/api/setup", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		r.PerformSetup(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d. Body: %s", w.Code, w.Body.String())
			return
		}

		// Check for auth_token cookie
		cookies := w.Result().Cookies()
		foundAuthCookie := false
		for _, c := range cookies {
			if c.Name == "auth_token" {
				foundAuthCookie = true
				if c.Value == "" {
					t.Error("auth_token cookie is empty")
				}
				break
			}
		}
		if !foundAuthCookie {
			t.Error("Expected auth_token cookie to be set for auto-login")
		}

		// Check response includes user info
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}
		if resp["success"] != true {
			t.Error("Expected success: true in response")
		}
		if resp["user"] == nil {
			t.Error("Expected user info in response")
		}
	})
}
