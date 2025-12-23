package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/clusteruptime/clusteruptime/internal/db"
	"github.com/clusteruptime/clusteruptime/internal/uptime"
	"github.com/go-chi/chi/v5"
)

func TestPerformSetup_Validation(t *testing.T) {
	s, _ := db.NewStore(":memory:")
	m := uptime.NewManager(s)
	r := &Router{Mux: chi.NewRouter(), manager: m, store: s}

	tests := []struct {
		name       string
		username   string
		password   string
		wantStatus int
	}{
		{
			name:       "Valid Setup",
			username:   "valid-user",
			password:   "Password123!",
			wantStatus: http.StatusOK,
		},
		{
			name:       "Username Too Long",
			username:   strings.Repeat("a", 33),
			password:   "Password123!",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Valid with Dot and Dash",
			username:   "valid.user-name",
			password:   "Password123!",
			wantStatus: http.StatusOK,
		},
		{
			name:       "Invalid characters (Space)",
			username:   "User Name",
			password:   "Password123!",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Invalid characters (Uppercase)",
			username:   "ValidUser",
			password:   "Password123!",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Password too short",
			username:   "valid",
			password:   "short",
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
			w := httptest.NewRecorder()

			r.PerformSetup(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Test %s: Expected status %d, got %d. Body: %s", tt.name, tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}
