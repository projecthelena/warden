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

func TestAuthMeIntegration(t *testing.T) {
	_, _, _, router, s := setupTest(t)

	// Setup User
	if err := s.CreateUser("admin", "correct-password", "UTC"); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// 1. Login to get cookie
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "correct-password"})
	reqLogin := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(body))
	wLogin := httptest.NewRecorder()
	router.ServeHTTP(wLogin, reqLogin)

	if wLogin.Code != http.StatusOK {
		t.Fatalf("Login failed, got %d", wLogin.Code)
	}

	cookies := wLogin.Result().Cookies()
	var authToken *http.Cookie
	for _, c := range cookies {
		if c.Name == "auth_token" {
			authToken = c
			break
		}
	}
	if authToken == nil {
		t.Fatal("No auth token cookie returned")
	}

	// 2. Call /api/auth/me (Protected)
	reqMe := httptest.NewRequest("GET", "/api/auth/me", nil)
	reqMe.AddCookie(authToken)
	wMe := httptest.NewRecorder()
	router.ServeHTTP(wMe, reqMe)

	if wMe.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for /api/auth/me, got %d. This confirms the context key mismatch bug.", wMe.Code)
	}

	// 3. Call PATCH /api/auth/me (Update Password - Missing Current)
	updatePayload := map[string]string{"password": "newpassword"}
	updateBody, _ := json.Marshal(updatePayload)
	reqUpdate := httptest.NewRequest("PATCH", "/api/auth/me", bytes.NewBuffer(updateBody))
	reqUpdate.AddCookie(authToken)
	wUpdate := httptest.NewRecorder()
	router.ServeHTTP(wUpdate, reqUpdate)

	if wUpdate.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request when missing current password, got %d", wUpdate.Code)
	}

	// 4. Call PATCH /api/auth/me (Update Password - Wrong Current)
	updatePayload2 := map[string]string{"password": "newpassword", "currentPassword": "wrong"}
	updateBody2, _ := json.Marshal(updatePayload2)
	reqUpdate2 := httptest.NewRequest("PATCH", "/api/auth/me", bytes.NewBuffer(updateBody2))
	reqUpdate2.AddCookie(authToken)
	wUpdate2 := httptest.NewRecorder()
	router.ServeHTTP(wUpdate2, reqUpdate2)

	if wUpdate2.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for wrong current password, got %d", wUpdate2.Code)
	}

	// 5. Call PATCH /api/auth/me (Update Password - Success)
	updatePayload3 := map[string]string{"password": "newpassword", "currentPassword": "correct-password"}
	updateBody3, _ := json.Marshal(updatePayload3)
	reqUpdate3 := httptest.NewRequest("PATCH", "/api/auth/me", bytes.NewBuffer(updateBody3))
	reqUpdate3.AddCookie(authToken)
	wUpdate3 := httptest.NewRecorder()
	router.ServeHTTP(wUpdate3, reqUpdate3)

	if wUpdate3.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for successful password update, got %d", wUpdate3.Code)
	}
}
