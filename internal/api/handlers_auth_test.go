package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthLogin(t *testing.T) {
	_, _, authH, _, s := setupTest(t)

	// Setup User
	if err := s.CreateUser("admin", "correct-password", "UTC"); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	tests := []struct {
		name       string
		payload    map[string]string
		wantStatus int
	}{
		{
			name:       "Success",
			payload:    map[string]string{"username": "admin", "password": "correct-password"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "Wrong Password",
			payload:    map[string]string{"username": "admin", "password": "wrong-password"},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "User Not Found",
			payload:    map[string]string{"username": "missing", "password": "password"},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			authH.Login(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Login() status = %v, want %v", w.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				// Check for Session Cookie
				cookies := w.Result().Cookies()
				found := false
				for _, c := range cookies {
					if c.Name == "auth_token" {
						found = true
						if !c.HttpOnly {
							t.Error("Session cookie should be HttpOnly")
						}
					}
				}
				if !found {
					t.Error("Session cookie not found on success")
				}
			}
		})
	}
}
