package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/uptime"
)

// ==================== Admin vs Editor boundary: admin-only endpoints ====================
// These tests validate that editors are BLOCKED from admin-only endpoints
// and that admins are ALLOWED.

func TestEditorCannotAccessAdminEndpoints(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	manager := uptime.NewManager(store)

	// Create users for handler tests that need user IDs
	_ = store.CreateUser("admin1", "pass", "UTC", "admin")
	_ = store.CreateUser("target1", "pass", "UTC", "viewer")
	admin, _ := store.Authenticate("admin1", "pass")
	target, _ := store.Authenticate("target1", "pass")

	settingsH := NewSettingsHandler(store, manager)
	userH := NewUserHandler(store)
	apiKeyH := NewAPIKeyHandler(store)

	tests := []struct {
		name         string
		method       string
		routePattern string
		requestPath  string
		handler      http.HandlerFunc
		body         any
	}{
		// Settings
		{
			name: "PATCH /api/settings", method: "PATCH",
			routePattern: "/api/settings", requestPath: "/api/settings",
			handler: settingsH.UpdateSettings, body: map[string]string{"data_retention_days": "90"},
		},
		// Users CRUD
		{
			name: "POST /api/users (create user)", method: "POST",
			routePattern: "/api/users", requestPath: "/api/users",
			handler: userH.CreateUser, body: map[string]string{"username": "newuser", "password": "longpassword", "role": "viewer"},
		},
		{
			name: "GET /api/users (list users)", method: "GET",
			routePattern: "/api/users", requestPath: "/api/users",
			handler: userH.ListUsers,
		},
		{
			name: "PATCH /api/users/{id}/role", method: "PATCH",
			routePattern: "/api/users/{id}/role", requestPath: fmt.Sprintf("/api/users/%d/role", target.ID),
			handler: userH.UpdateUserRole, body: map[string]string{"role": "editor"},
		},
		{
			name: "DELETE /api/users/{id}", method: "DELETE",
			routePattern: "/api/users/{id}", requestPath: fmt.Sprintf("/api/users/%d", target.ID),
			handler: userH.DeleteUser,
		},
		{
			name: "GET /api/users/{id}/status-pages", method: "GET",
			routePattern: "/api/users/{id}/status-pages", requestPath: fmt.Sprintf("/api/users/%d/status-pages", target.ID),
			handler: userH.GetUserStatusPages,
		},
		{
			name: "PUT /api/users/{id}/status-pages", method: "PUT",
			routePattern: "/api/users/{id}/status-pages", requestPath: fmt.Sprintf("/api/users/%d/status-pages", target.ID),
			handler: userH.SetUserStatusPages, body: map[string]any{"statusPageIds": []int64{}},
		},
		// API Keys
		{
			name: "GET /api-keys (list keys)", method: "GET",
			routePattern: "/api/api-keys", requestPath: "/api/api-keys",
			handler: apiKeyH.ListKeys,
		},
		{
			name: "POST /api-keys (create key)", method: "POST",
			routePattern: "/api/api-keys", requestPath: "/api/api-keys",
			handler: apiKeyH.CreateKey, body: map[string]string{"name": "test-key"},
		},
		{
			name: "DELETE /api-keys/{id}", method: "DELETE",
			routePattern: "/api/api-keys/{id}", requestPath: "/api/api-keys/1",
			handler: apiKeyH.DeleteKey,
		},
	}

	for _, tc := range tests {
		t.Run("editor_blocked_"+tc.name, func(t *testing.T) {
			var bodyReader *bytes.Buffer
			if tc.body != nil {
				b, _ := json.Marshal(tc.body)
				bodyReader = bytes.NewBuffer(b)
			} else {
				bodyReader = bytes.NewBuffer(nil)
			}

			r := chi.NewRouter()
			r.Use(roleAndUserMiddleware(RoleEditor, admin.ID))

			switch tc.method {
			case "GET":
				r.Get(tc.routePattern, tc.handler)
			case "POST":
				r.Post(tc.routePattern, tc.handler)
			case "PATCH":
				r.Patch(tc.routePattern, tc.handler)
			case "PUT":
				r.Put(tc.routePattern, tc.handler)
			case "DELETE":
				r.Delete(tc.routePattern, tc.handler)
			}

			req := httptest.NewRequest(tc.method, tc.requestPath, bodyReader)
			if tc.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("Editor should be forbidden from %s %s: expected 403, got %d. Body: %s",
					tc.method, tc.requestPath, w.Code, w.Body.String())
			}
		})
	}
}

func TestAdminCanAccessAdminEndpoints(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	manager := uptime.NewManager(store)

	// Create users for handler tests
	_ = store.CreateUser("admin1", "pass", "UTC", "admin")
	_ = store.CreateUser("admin2", "pass", "UTC", "admin")
	_ = store.CreateUser("target1", "pass", "UTC", "viewer")
	admin1, _ := store.Authenticate("admin1", "pass")
	target, _ := store.Authenticate("target1", "pass")

	settingsH := NewSettingsHandler(store, manager)
	userH := NewUserHandler(store)
	apiKeyH := NewAPIKeyHandler(store)

	tests := []struct {
		name         string
		method       string
		routePattern string // chi route pattern (with {id} params)
		requestPath  string // actual request URL
		handler      http.HandlerFunc
		body         any
		wantStatus   int
	}{
		{
			name: "PATCH /api/settings", method: "PATCH",
			routePattern: "/api/settings", requestPath: "/api/settings",
			handler: settingsH.UpdateSettings, body: map[string]string{"data_retention_days": "90"},
			wantStatus: http.StatusOK,
		},
		{
			name: "POST /api/users", method: "POST",
			routePattern: "/api/users", requestPath: "/api/users",
			handler: userH.CreateUser, body: map[string]string{"username": "newadminuser", "password": "longpassword", "role": "viewer"},
			wantStatus: http.StatusCreated,
		},
		{
			name: "GET /api/users", method: "GET",
			routePattern: "/api/users", requestPath: "/api/users",
			handler: userH.ListUsers, wantStatus: http.StatusOK,
		},
		{
			name: "PATCH /api/users/{id}/role", method: "PATCH",
			routePattern: "/api/users/{id}/role", requestPath: fmt.Sprintf("/api/users/%d/role", target.ID),
			handler: userH.UpdateUserRole, body: map[string]string{"role": "editor"},
			wantStatus: http.StatusOK,
		},
		{
			name: "GET /api/users/{id}/status-pages", method: "GET",
			routePattern: "/api/users/{id}/status-pages", requestPath: fmt.Sprintf("/api/users/%d/status-pages", target.ID),
			handler: userH.GetUserStatusPages, wantStatus: http.StatusOK,
		},
		{
			name: "PUT /api/users/{id}/status-pages", method: "PUT",
			routePattern: "/api/users/{id}/status-pages", requestPath: fmt.Sprintf("/api/users/%d/status-pages", target.ID),
			handler: userH.SetUserStatusPages, body: map[string]any{"statusPageIds": []int64{}},
			wantStatus: http.StatusOK,
		},
		{
			name: "GET /api-keys", method: "GET",
			routePattern: "/api/api-keys", requestPath: "/api/api-keys",
			handler: apiKeyH.ListKeys, wantStatus: http.StatusOK,
		},
		{
			name: "POST /api-keys", method: "POST",
			routePattern: "/api/api-keys", requestPath: "/api/api-keys",
			handler: apiKeyH.CreateKey, body: map[string]string{"name": "admin-test-key"},
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run("admin_allowed_"+tc.name, func(t *testing.T) {
			var bodyReader *bytes.Buffer
			if tc.body != nil {
				b, _ := json.Marshal(tc.body)
				bodyReader = bytes.NewBuffer(b)
			} else {
				bodyReader = bytes.NewBuffer(nil)
			}

			r := chi.NewRouter()
			r.Use(roleAndUserMiddleware(RoleAdmin, admin1.ID))

			switch tc.method {
			case "GET":
				r.Get(tc.routePattern, tc.handler)
			case "POST":
				r.Post(tc.routePattern, tc.handler)
			case "PATCH":
				r.Patch(tc.routePattern, tc.handler)
			case "PUT":
				r.Put(tc.routePattern, tc.handler)
			case "DELETE":
				r.Delete(tc.routePattern, tc.handler)
			}

			req := httptest.NewRequest(tc.method, tc.requestPath, bodyReader)
			if tc.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("Admin should be allowed %s %s: expected %d, got %d. Body: %s",
					tc.method, tc.requestPath, tc.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

// ==================== Admin vs Editor boundary: editor-level endpoints ====================
// These tests validate that editors CAN access editor-level endpoints
// and that admins can also access them (hierarchy).

func TestEditorCanAccessEditorEndpoints(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	manager := uptime.NewManager(store)

	crudH := NewCRUDHandler(store, manager)
	incidentH := NewIncidentHandler(store)
	maintH := NewMaintenanceHandler(store, manager)
	notifH := NewNotificationChannelsHandler(store)

	// ── Groups ──
	t.Run("editor can create group", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"name": "Editor Group"})
		r := chi.NewRouter()
		r.Use(roleMiddleware(RoleEditor))
		r.Post("/api/groups", crudH.CreateGroup)

		req := httptest.NewRequest("POST", "/api/groups", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected 201, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("editor can create monitor", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"name":     "Editor Monitor",
			"url":      "https://example.com",
			"groupId":  "g-editor-group",
			"interval": 60,
		})
		r := chi.NewRouter()
		r.Use(roleMiddleware(RoleEditor))
		r.Post("/api/monitors", crudH.CreateMonitor)

		req := httptest.NewRequest("POST", "/api/monitors", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected 201, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	// ── Incidents ──
	t.Run("editor can create incident", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"title":          "Test Incident",
			"description":    "Something happened",
			"severity":       "major",
			"status":         "investigating",
			"startTime":      time.Now().UTC().Format(time.RFC3339),
			"affectedGroups": []string{},
			"public":         true,
		})
		r := chi.NewRouter()
		r.Use(roleMiddleware(RoleEditor))
		r.Post("/api/incidents", incidentH.CreateIncident)

		req := httptest.NewRequest("POST", "/api/incidents", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected 201, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	// ── Maintenance ──
	t.Run("editor can create maintenance", func(t *testing.T) {
		start := time.Now().Add(1 * time.Hour).UTC()
		end := time.Now().Add(2 * time.Hour).UTC()
		body, _ := json.Marshal(map[string]any{
			"title":          "Maintenance Window",
			"description":    "Scheduled maintenance",
			"status":         "scheduled",
			"startTime":      start.Format(time.RFC3339),
			"endTime":        end.Format(time.RFC3339),
			"affectedGroups": []string{},
		})
		r := chi.NewRouter()
		r.Use(roleMiddleware(RoleEditor))
		r.Post("/api/maintenance", maintH.CreateMaintenance)

		req := httptest.NewRequest("POST", "/api/maintenance", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected 201, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	// ── Notification Channels ──
	t.Run("editor can create notification channel", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"type":    "webhook",
			"name":    "Editor Webhook",
			"config":  map[string]any{"webhook_url": "https://example.com/hook"},
			"enabled": true,
		})
		r := chi.NewRouter()
		r.Use(roleMiddleware(RoleEditor))
		r.Post("/api/notifications/channels", notifH.CreateChannel)

		req := httptest.NewRequest("POST", "/api/notifications/channels", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected 201, got %d. Body: %s", w.Code, w.Body.String())
		}
	})
}

func TestAdminCanAccessEditorEndpoints(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	manager := uptime.NewManager(store)

	crudH := NewCRUDHandler(store, manager)
	incidentH := NewIncidentHandler(store)
	maintH := NewMaintenanceHandler(store, manager)
	notifH := NewNotificationChannelsHandler(store)

	// ── Groups ──
	t.Run("admin can create group", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"name": "Admin Group"})
		r := chi.NewRouter()
		r.Use(roleMiddleware(RoleAdmin))
		r.Post("/api/groups", crudH.CreateGroup)

		req := httptest.NewRequest("POST", "/api/groups", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected 201, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("admin can create monitor", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"name":     "Admin Monitor",
			"url":      "https://example.com",
			"groupId":  "g-admin-group",
			"interval": 60,
		})
		r := chi.NewRouter()
		r.Use(roleMiddleware(RoleAdmin))
		r.Post("/api/monitors", crudH.CreateMonitor)

		req := httptest.NewRequest("POST", "/api/monitors", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected 201, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	// ── Incidents ──
	t.Run("admin can create incident", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"title":          "Admin Incident",
			"description":    "Admin created this",
			"severity":       "minor",
			"status":         "investigating",
			"startTime":      time.Now().UTC().Format(time.RFC3339),
			"affectedGroups": []string{},
			"public":         false,
		})
		r := chi.NewRouter()
		r.Use(roleMiddleware(RoleAdmin))
		r.Post("/api/incidents", incidentH.CreateIncident)

		req := httptest.NewRequest("POST", "/api/incidents", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected 201, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	// ── Maintenance ──
	t.Run("admin can create maintenance", func(t *testing.T) {
		start := time.Now().Add(1 * time.Hour).UTC()
		end := time.Now().Add(2 * time.Hour).UTC()
		body, _ := json.Marshal(map[string]any{
			"title":          "Admin Maintenance",
			"description":    "Admin scheduled",
			"status":         "scheduled",
			"startTime":      start.Format(time.RFC3339),
			"endTime":        end.Format(time.RFC3339),
			"affectedGroups": []string{},
		})
		r := chi.NewRouter()
		r.Use(roleMiddleware(RoleAdmin))
		r.Post("/api/maintenance", maintH.CreateMaintenance)

		req := httptest.NewRequest("POST", "/api/maintenance", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected 201, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	// ── Notification Channels ──
	t.Run("admin can create notification channel", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"type":    "webhook",
			"name":    "Admin Webhook",
			"config":  map[string]any{"webhook_url": "https://example.com/admin-hook"},
			"enabled": true,
		})
		r := chi.NewRouter()
		r.Use(roleMiddleware(RoleAdmin))
		r.Post("/api/notifications/channels", notifH.CreateChannel)

		req := httptest.NewRequest("POST", "/api/notifications/channels", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected 201, got %d. Body: %s", w.Code, w.Body.String())
		}
	})
}

// ==================== Comprehensive permission matrix ====================
// Table-driven test covering every role x endpoint combination at the admin/editor boundary.
// Uses bodyFn to generate unique names per role, avoiding name conflicts.

func TestAdminEditorPermissionMatrix(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	manager := uptime.NewManager(store)

	_ = store.CreateUser("admin_matrix", "pass", "UTC", "admin")
	_ = store.CreateUser("target_matrix", "pass", "UTC", "viewer")
	admin, _ := store.Authenticate("admin_matrix", "pass")

	crudH := NewCRUDHandler(store, manager)
	settingsH := NewSettingsHandler(store, manager)
	userH := NewUserHandler(store)
	apiKeyH := NewAPIKeyHandler(store)
	incidentH := NewIncidentHandler(store)
	maintH := NewMaintenanceHandler(store, manager)
	notifH := NewNotificationChannelsHandler(store)

	// Pre-create a group so monitors can reference it
	{
		body, _ := json.Marshal(map[string]string{"name": "Matrix Group"})
		r := chi.NewRouter()
		r.Use(roleMiddleware(RoleAdmin))
		r.Post("/api/groups", crudH.CreateGroup)
		req := httptest.NewRequest("POST", "/api/groups", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}

	// bodyFn generates a request body with role-specific names to avoid conflicts.
	type bodyFn func(role string) any

	type testCase struct {
		name       string
		method     string
		path       string
		handler    http.HandlerFunc
		bodyFn     bodyFn
		editorWant int
		adminWant  int
	}

	start := time.Now().Add(1 * time.Hour).UTC()
	end := time.Now().Add(2 * time.Hour).UTC()

	tests := []testCase{
		// ── Editor-level endpoints (editor=allow, admin=allow) ──
		{
			name: "create group", method: "POST", path: "/api/groups",
			handler:    crudH.CreateGroup,
			bodyFn:     func(role string) any { return map[string]string{"name": "Mx " + role + " Group"} },
			editorWant: http.StatusCreated, adminWant: http.StatusCreated,
		},
		{
			name: "create monitor", method: "POST", path: "/api/monitors",
			handler: crudH.CreateMonitor,
			bodyFn: func(role string) any {
				return map[string]any{
					"name": "Mx " + role + " Monitor", "url": "https://example.com",
					"groupId": "g-matrix-group", "interval": 60,
				}
			},
			editorWant: http.StatusCreated, adminWant: http.StatusCreated,
		},
		{
			name: "create incident", method: "POST", path: "/api/incidents",
			handler: incidentH.CreateIncident,
			bodyFn: func(_ string) any {
				return map[string]any{
					"title": "Mx Incident", "description": "test",
					"severity": "major", "status": "investigating",
					"startTime": time.Now().UTC().Format(time.RFC3339), "affectedGroups": []string{}, "public": true,
				}
			},
			editorWant: http.StatusCreated, adminWant: http.StatusCreated,
		},
		{
			name: "create maintenance", method: "POST", path: "/api/maintenance",
			handler: maintH.CreateMaintenance,
			bodyFn: func(_ string) any {
				return map[string]any{
					"title": "Mx Maintenance", "description": "test",
					"status": "scheduled", "startTime": start.Format(time.RFC3339),
					"endTime": end.Format(time.RFC3339), "affectedGroups": []string{},
				}
			},
			editorWant: http.StatusCreated, adminWant: http.StatusCreated,
		},
		{
			name: "create notification channel", method: "POST", path: "/api/notifications/channels",
			handler: notifH.CreateChannel,
			bodyFn: func(role string) any {
				return map[string]any{
					"type": "webhook", "name": "Mx " + role + " Webhook",
					"config": map[string]any{"webhook_url": "https://example.com/hook"}, "enabled": true,
				}
			},
			editorWant: http.StatusCreated, adminWant: http.StatusCreated,
		},

		// ── Admin-only endpoints (editor=403, admin=allow) ──
		{
			name: "update settings", method: "PATCH", path: "/api/settings",
			handler:    settingsH.UpdateSettings,
			bodyFn:     func(_ string) any { return map[string]string{"data_retention_days": "90"} },
			editorWant: http.StatusForbidden, adminWant: http.StatusOK,
		},
		{
			name: "list users", method: "GET", path: "/api/users",
			handler:    userH.ListUsers,
			bodyFn:     nil,
			editorWant: http.StatusForbidden, adminWant: http.StatusOK,
		},
		{
			name: "create user", method: "POST", path: "/api/users",
			handler: userH.CreateUser,
			bodyFn: func(role string) any {
				return map[string]string{"username": "mx" + role, "password": "longpassword", "role": "viewer"}
			},
			editorWant: http.StatusForbidden, adminWant: http.StatusCreated,
		},
		{
			name: "list api keys", method: "GET", path: "/api/api-keys",
			handler:    apiKeyH.ListKeys,
			bodyFn:     nil,
			editorWant: http.StatusForbidden, adminWant: http.StatusOK,
		},
		{
			name: "create api key", method: "POST", path: "/api/api-keys",
			handler: apiKeyH.CreateKey,
			bodyFn: func(role string) any {
				return map[string]string{"name": "mx-" + role + "-key"}
			},
			editorWant: http.StatusForbidden, adminWant: http.StatusOK,
		},
	}

	runTest := func(t *testing.T, tc testCase, role string, wantStatus int) {
		t.Helper()
		var bodyReader *bytes.Buffer
		if tc.bodyFn != nil {
			b, _ := json.Marshal(tc.bodyFn(role))
			bodyReader = bytes.NewBuffer(b)
		} else {
			bodyReader = bytes.NewBuffer(nil)
		}

		r := chi.NewRouter()
		r.Use(roleAndUserMiddleware(role, admin.ID))
		switch tc.method {
		case "GET":
			r.Get(tc.path, tc.handler)
		case "POST":
			r.Post(tc.path, tc.handler)
		case "PATCH":
			r.Patch(tc.path, tc.handler)
		case "PUT":
			r.Put(tc.path, tc.handler)
		case "DELETE":
			r.Delete(tc.path, tc.handler)
		}

		req := httptest.NewRequest(tc.method, tc.path, bodyReader)
		if tc.bodyFn != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != wantStatus {
			t.Errorf("%s %s %s: expected %d, got %d. Body: %s",
				role, tc.method, tc.path, wantStatus, w.Code, w.Body.String())
		}
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("editor/%s", tc.name), func(t *testing.T) {
			runTest(t, tc, RoleEditor, tc.editorWant)
		})
		t.Run(fmt.Sprintf("admin/%s", tc.name), func(t *testing.T) {
			runTest(t, tc, RoleAdmin, tc.adminWant)
		})
	}
}

// ==================== Viewer blocked from editor endpoints ====================
// Ensures the editor level is a real boundary (viewer cannot do what editor can).

func TestViewerCannotAccessEditorEndpoints(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	manager := uptime.NewManager(store)

	crudH := NewCRUDHandler(store, manager)
	incidentH := NewIncidentHandler(store)
	maintH := NewMaintenanceHandler(store, manager)
	notifH := NewNotificationChannelsHandler(store)

	start := time.Now().Add(1 * time.Hour).UTC()
	end := time.Now().Add(2 * time.Hour).UTC()

	tests := []struct {
		name    string
		method  string
		path    string
		handler http.HandlerFunc
		body    any
	}{
		{
			name: "create group", method: "POST", path: "/api/groups",
			handler: crudH.CreateGroup,
			body:    map[string]string{"name": "Viewer Group"},
		},
		{
			name: "create monitor", method: "POST", path: "/api/monitors",
			handler: crudH.CreateMonitor,
			body: map[string]any{
				"name": "Viewer Monitor", "url": "https://example.com",
				"groupId": "g-default", "interval": 60,
			},
		},
		{
			name: "create incident", method: "POST", path: "/api/incidents",
			handler: incidentH.CreateIncident,
			body: map[string]any{
				"title": "Viewer Incident", "description": "test",
				"severity": "major", "status": "investigating",
				"startTime": time.Now().UTC().Format(time.RFC3339), "affectedGroups": []string{}, "public": true,
			},
		},
		{
			name: "create maintenance", method: "POST", path: "/api/maintenance",
			handler: maintH.CreateMaintenance,
			body: map[string]any{
				"title": "Viewer Maintenance", "description": "test",
				"status": "scheduled", "startTime": start.Format(time.RFC3339),
				"endTime": end.Format(time.RFC3339), "affectedGroups": []string{},
			},
		},
		{
			name: "create notification channel", method: "POST", path: "/api/notifications/channels",
			handler: notifH.CreateChannel,
			body: map[string]any{
				"type": "webhook", "name": "Viewer Webhook",
				"config": map[string]any{"webhook_url": "https://example.com/hook"}, "enabled": true,
			},
		},
	}

	for _, tc := range tests {
		t.Run("viewer_blocked_"+tc.name, func(t *testing.T) {
			b, _ := json.Marshal(tc.body)
			r := chi.NewRouter()
			r.Use(roleMiddleware(RoleViewer))
			switch tc.method {
			case "POST":
				r.Post(tc.path, tc.handler)
			case "PUT":
				r.Put(tc.path, tc.handler)
			case "PATCH":
				r.Patch(tc.path, tc.handler)
			case "DELETE":
				r.Delete(tc.path, tc.handler)
			}

			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("Viewer should be forbidden from %s: expected 403, got %d. Body: %s",
					tc.name, w.Code, w.Body.String())
			}
		})
	}
}
