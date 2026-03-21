package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/uptime"
)

// roleMiddleware is a test middleware that injects a specific role into context.
func roleMiddleware(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), contextKeyUserRole, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// roleAndUserMiddleware injects both role and user ID into context.
func roleAndUserMiddleware(role string, userID int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), contextKeyUserRole, role)
			ctx = context.WithValue(ctx, contextKeyUserID, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ==================== requireRole tests ====================

func TestRequireRole_AdminCanAccessAdminEndpoint(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requireRole(w, r, RoleAdmin) {
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r := chi.NewRouter()
	r.Use(roleMiddleware(RoleAdmin))
	r.Get("/test", handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestRequireRole_EditorCanAccessEditorEndpoint(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requireRole(w, r, RoleEditor) {
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	r := chi.NewRouter()
	r.Use(roleMiddleware(RoleEditor))
	r.Get("/test", handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestRequireRole_AdminCanAccessEditorEndpoint(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requireRole(w, r, RoleEditor) {
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	r := chi.NewRouter()
	r.Use(roleMiddleware(RoleAdmin))
	r.Get("/test", handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestRequireRole_ViewerCannotAccessEditorEndpoint(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requireRole(w, r, RoleEditor) {
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	r := chi.NewRouter()
	r.Use(roleMiddleware(RoleViewer))
	r.Get("/test", handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", w.Code)
	}
}

func TestRequireRole_StatusViewerCannotAccessViewerEndpoint(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requireRole(w, r, RoleViewer) {
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	r := chi.NewRouter()
	r.Use(roleMiddleware(RoleStatusViewer))
	r.Get("/test", handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", w.Code)
	}
}

func TestRequireRole_EmptyRoleGetsForbidden(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requireRole(w, r, RoleViewer) {
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	r := chi.NewRouter()
	r.Use(roleMiddleware(""))
	r.Get("/test", handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", w.Code)
	}
}

func TestRequireRole_NoRoleInContext(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requireRole(w, r, RoleViewer) {
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// No middleware to inject role
	r := chi.NewRouter()
	r.Get("/test", handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403 with no role in context, got %d", w.Code)
	}
}

// ==================== hasPermission tests ====================

func TestHasPermission(t *testing.T) {
	tests := []struct {
		userRole    string
		minRole     string
		expected    bool
		description string
	}{
		{RoleAdmin, RoleAdmin, true, "admin >= admin"},
		{RoleAdmin, RoleEditor, true, "admin >= editor"},
		{RoleAdmin, RoleViewer, true, "admin >= viewer"},
		{RoleAdmin, RoleStatusViewer, true, "admin >= status_viewer"},
		{RoleEditor, RoleEditor, true, "editor >= editor"},
		{RoleEditor, RoleViewer, true, "editor >= viewer"},
		{RoleEditor, RoleStatusViewer, true, "editor >= status_viewer"},
		{RoleEditor, RoleAdmin, false, "editor < admin"},
		{RoleViewer, RoleViewer, true, "viewer >= viewer"},
		{RoleViewer, RoleStatusViewer, true, "viewer >= status_viewer"},
		{RoleViewer, RoleEditor, false, "viewer < editor"},
		{RoleViewer, RoleAdmin, false, "viewer < admin"},
		{RoleStatusViewer, RoleStatusViewer, true, "status_viewer >= status_viewer"},
		{RoleStatusViewer, RoleViewer, false, "status_viewer < viewer"},
		{RoleStatusViewer, RoleEditor, false, "status_viewer < editor"},
		{RoleStatusViewer, RoleAdmin, false, "status_viewer < admin"},
		{"", RoleStatusViewer, false, "empty < status_viewer"},
		{"", RoleViewer, false, "empty < viewer"},
		{"invalid_role", RoleViewer, false, "invalid < viewer"},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			result := hasPermission(tc.userRole, tc.minRole)
			if result != tc.expected {
				t.Errorf("hasPermission(%q, %q) = %v, want %v", tc.userRole, tc.minRole, result, tc.expected)
			}
		})
	}
}

// ==================== ValidRole tests ====================

func TestValidRole(t *testing.T) {
	validRoles := []string{"admin", "editor", "viewer", "status_viewer"}
	for _, role := range validRoles {
		t.Run(fmt.Sprintf("valid_%s", role), func(t *testing.T) {
			if !ValidRole(role) {
				t.Errorf("ValidRole(%q) = false, want true", role)
			}
		})
	}

	invalidRoles := []string{"", "superadmin", "Admin", "ADMIN", "root", "user", "moderator", "status-viewer"}
	for _, role := range invalidRoles {
		t.Run(fmt.Sprintf("invalid_%s", role), func(t *testing.T) {
			if ValidRole(role) {
				t.Errorf("ValidRole(%q) = true, want false", role)
			}
		})
	}
}

// ==================== RequireViewerMiddleware tests ====================

func TestRequireViewerMiddleware_BlocksStatusViewer(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r := chi.NewRouter()
	r.Use(roleMiddleware(RoleStatusViewer))
	r.Use(RequireViewerMiddleware)
	r.Get("/test", handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for status_viewer, got %d", w.Code)
	}
}

func TestRequireViewerMiddleware_AllowsViewer(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r := chi.NewRouter()
	r.Use(roleMiddleware(RoleViewer))
	r.Use(RequireViewerMiddleware)
	r.Get("/test", handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 for viewer, got %d", w.Code)
	}
}

func TestRequireViewerMiddleware_AllowsEditor(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r := chi.NewRouter()
	r.Use(roleMiddleware(RoleEditor))
	r.Use(RequireViewerMiddleware)
	r.Get("/test", handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 for editor, got %d", w.Code)
	}
}

func TestRequireViewerMiddleware_AllowsAdmin(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r := chi.NewRouter()
	r.Use(roleMiddleware(RoleAdmin))
	r.Use(RequireViewerMiddleware)
	r.Get("/test", handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 for admin, got %d", w.Code)
	}
}

// ==================== Handler role enforcement (integration-style) ====================

func TestViewerCanGetOverview(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	manager := uptime.NewManager(store)
	uptimeH := NewUptimeHandler(manager, store)

	r := chi.NewRouter()
	r.Use(roleMiddleware(RoleViewer))
	r.Use(RequireViewerMiddleware)
	r.Get("/api/overview", uptimeH.GetOverview)

	req := httptest.NewRequest("GET", "/api/overview", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected viewer to GET /api/overview (200), got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestViewerCannotPostMonitors(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	manager := uptime.NewManager(store)
	crudH := NewCRUDHandler(store, manager)

	payload := map[string]interface{}{
		"name":     "Test Monitor",
		"url":      "http://test.com",
		"groupId":  "g-default",
		"interval": 60,
	}
	body, _ := json.Marshal(payload)

	r := chi.NewRouter()
	r.Use(roleMiddleware(RoleViewer))
	r.Post("/api/monitors", crudH.CreateMonitor)

	req := httptest.NewRequest("POST", "/api/monitors", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected viewer to be forbidden from POST /api/monitors (403), got %d", w.Code)
	}
}

func TestEditorCanPostMonitors(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	manager := uptime.NewManager(store)
	crudH := NewCRUDHandler(store, manager)

	payload := map[string]interface{}{
		"name":     "Editor Monitor",
		"url":      "http://editor-test.com",
		"groupId":  "g-default",
		"interval": 60,
	}
	body, _ := json.Marshal(payload)

	r := chi.NewRouter()
	r.Use(roleMiddleware(RoleEditor))
	r.Post("/api/monitors", crudH.CreateMonitor)

	req := httptest.NewRequest("POST", "/api/monitors", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected editor to POST /api/monitors (201), got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestViewerCannotPatchSettings(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	manager := uptime.NewManager(store)
	settingsH := NewSettingsHandler(store, manager)

	payload := map[string]string{"data_retention_days": "90"}
	body, _ := json.Marshal(payload)

	r := chi.NewRouter()
	r.Use(roleMiddleware(RoleViewer))
	r.Patch("/api/settings", settingsH.UpdateSettings)

	req := httptest.NewRequest("PATCH", "/api/settings", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected viewer to be forbidden from PATCH /api/settings (403), got %d", w.Code)
	}
}

func TestEditorCannotPatchSettings(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	manager := uptime.NewManager(store)
	settingsH := NewSettingsHandler(store, manager)

	payload := map[string]string{"data_retention_days": "90"}
	body, _ := json.Marshal(payload)

	r := chi.NewRouter()
	r.Use(roleMiddleware(RoleEditor))
	r.Patch("/api/settings", settingsH.UpdateSettings)

	req := httptest.NewRequest("PATCH", "/api/settings", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected editor to be forbidden from PATCH /api/settings (403), got %d", w.Code)
	}
}

func TestAdminCanPatchSettings(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	manager := uptime.NewManager(store)
	settingsH := NewSettingsHandler(store, manager)

	payload := map[string]string{"data_retention_days": "90"}
	body, _ := json.Marshal(payload)

	r := chi.NewRouter()
	r.Use(roleMiddleware(RoleAdmin))
	r.Patch("/api/settings", settingsH.UpdateSettings)

	req := httptest.NewRequest("PATCH", "/api/settings", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected admin to PATCH /api/settings (200), got %d. Body: %s", w.Code, w.Body.String())
	}
}

// ==================== User management handler tests ====================

func TestListUsers_ReturnsUsersWithRoles(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	// Create users with different roles
	_ = store.CreateUser("admin1", "pass", "UTC", "admin")
	_ = store.CreateUser("editor1", "pass", "UTC", "editor")
	_ = store.CreateUser("viewer1", "pass", "UTC", "viewer")

	userH := NewUserHandler(store)

	r := chi.NewRouter()
	r.Use(roleMiddleware(RoleAdmin))
	r.Get("/api/users", userH.ListUsers)

	req := httptest.NewRequest("GET", "/api/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Users []struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
			Role     string `json:"role"`
		} `json:"users"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(resp.Users) != 3 {
		t.Fatalf("Expected 3 users, got %d", len(resp.Users))
	}

	roleMap := make(map[string]string)
	for _, u := range resp.Users {
		roleMap[u.Username] = u.Role
	}

	if roleMap["admin1"] != "admin" {
		t.Errorf("admin1: expected role 'admin', got %s", roleMap["admin1"])
	}
	if roleMap["editor1"] != "editor" {
		t.Errorf("editor1: expected role 'editor', got %s", roleMap["editor1"])
	}
	if roleMap["viewer1"] != "viewer" {
		t.Errorf("viewer1: expected role 'viewer', got %s", roleMap["viewer1"])
	}
}

func TestListUsers_NonAdminForbidden(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	userH := NewUserHandler(store)

	roles := []string{RoleEditor, RoleViewer, RoleStatusViewer}
	for _, role := range roles {
		t.Run(role, func(t *testing.T) {
			r := chi.NewRouter()
			r.Use(roleMiddleware(role))
			r.Get("/api/users", userH.ListUsers)

			req := httptest.NewRequest("GET", "/api/users", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("Expected 403 for role %s, got %d", role, w.Code)
			}
		})
	}
}

func TestUpdateUserRole_ChangesRole(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	// Create admin (the requester, ID=1) and a target user
	_ = store.CreateUser("admin_requester", "pass", "UTC", "admin")
	_ = store.CreateUser("target_user", "pass", "UTC", "viewer")

	admin, _ := store.Authenticate("admin_requester", "pass")
	target, _ := store.Authenticate("target_user", "pass")

	userH := NewUserHandler(store)

	payload := map[string]string{"role": "editor"}
	body, _ := json.Marshal(payload)

	r := chi.NewRouter()
	r.Use(roleAndUserMiddleware(RoleAdmin, admin.ID))
	r.Patch("/api/users/{id}/role", userH.UpdateUserRole)

	req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/users/%d/role", target.ID), bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify role changed
	role, _ := store.GetUserRole(target.ID)
	if role != "editor" {
		t.Errorf("Expected role 'editor' after update, got %s", role)
	}
}

func TestUpdateUserRole_RejectsSelfChange(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	_ = store.CreateUser("self_admin", "pass", "UTC", "admin")
	admin, _ := store.Authenticate("self_admin", "pass")

	userH := NewUserHandler(store)

	payload := map[string]string{"role": "viewer"}
	body, _ := json.Marshal(payload)

	r := chi.NewRouter()
	r.Use(roleAndUserMiddleware(RoleAdmin, admin.ID))
	r.Patch("/api/users/{id}/role", userH.UpdateUserRole)

	// Try to change own role
	req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/users/%d/role", admin.ID), bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for self-change, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "cannot change your own role" {
		t.Errorf("Expected error 'cannot change your own role', got %s", resp["error"])
	}
}

func TestUpdateUserRole_PreventsRemovingLastAdmin(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	// Create the only admin and a separate requester admin
	// We need two admins so the requester admin can try to demote the other one
	// But if we only have one admin and try to demote them... let's test with two admins
	// and then try to demote both to leave zero.

	_ = store.CreateUser("admin_a", "pass", "UTC", "admin")
	_ = store.CreateUser("admin_b", "pass", "UTC", "admin")
	adminA, _ := store.Authenticate("admin_a", "pass")
	adminB, _ := store.Authenticate("admin_b", "pass")

	userH := NewUserHandler(store)

	// First, demote admin_b to editor (should succeed, admin_a remains)
	payload := map[string]string{"role": "editor"}
	body, _ := json.Marshal(payload)

	r := chi.NewRouter()
	r.Use(roleAndUserMiddleware(RoleAdmin, adminA.ID))
	r.Patch("/api/users/{id}/role", userH.UpdateUserRole)

	req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/users/%d/role", adminB.ID), bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 for demoting admin_b (admin_a remains), got %d. Body: %s", w.Code, w.Body.String())
	}

	// Now admin_a is the only admin. Create a new viewer to try to demote admin_a through them.
	// Actually, we cannot demote admin_a since the self-check prevents it.
	// Let's create a new admin_c and try to be the requester as admin_c demoting admin_a (the last admin).
	// Wait - admin_a is the last admin, admin_b is now editor.
	// We need a different admin to call the endpoint. Let's promote admin_b back temporarily.

	// Simpler approach: test with fresh data
	store2, _ := db.NewStore(db.NewTestConfig())
	_ = store2.CreateUser("solo_admin", "pass", "UTC", "admin")
	_ = store2.CreateUser("other_user", "pass", "UTC", "editor")
	soloAdmin, _ := store2.Authenticate("solo_admin", "pass")
	other, _ := store2.Authenticate("other_user", "pass")

	// Promote other to admin first so we have two admins, then demote solo_admin
	_ = store2.UpdateUserRole(other.ID, "admin")

	userH2 := NewUserHandler(store2)

	// Now demote other (leaving solo_admin as only admin) - should succeed
	payload2 := map[string]string{"role": "viewer"}
	body2, _ := json.Marshal(payload2)

	r2 := chi.NewRouter()
	r2.Use(roleAndUserMiddleware(RoleAdmin, soloAdmin.ID))
	r2.Patch("/api/users/{id}/role", userH2.UpdateUserRole)

	req2 := httptest.NewRequest("PATCH", fmt.Sprintf("/api/users/%d/role", other.ID), bytes.NewBuffer(body2))
	w2 := httptest.NewRecorder()
	r2.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w2.Code)
	}

	// Now solo_admin is the ONLY admin. Try to have someone else demote them.
	// We need a requester who is admin - but solo_admin is the only one and can't demote self.
	// The practical test: create a scenario where the endpoint is called for the last admin.
	// We'll use other_user as a "hacked" admin context to try to demote solo_admin.
	r3 := chi.NewRouter()
	r3.Use(roleAndUserMiddleware(RoleAdmin, other.ID)) // pretend other is admin in context
	r3.Patch("/api/users/{id}/role", userH2.UpdateUserRole)

	payload3 := map[string]string{"role": "editor"}
	body3, _ := json.Marshal(payload3)
	req3 := httptest.NewRequest("PATCH", fmt.Sprintf("/api/users/%d/role", soloAdmin.ID), bytes.NewBuffer(body3))
	w3 := httptest.NewRecorder()
	r3.ServeHTTP(w3, req3)

	if w3.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 when removing last admin, got %d. Body: %s", w3.Code, w3.Body.String())
	}

	var resp map[string]string
	_ = json.Unmarshal(w3.Body.Bytes(), &resp)
	if resp["error"] != "cannot remove the last admin" {
		t.Errorf("Expected error 'cannot remove the last admin', got %s", resp["error"])
	}
}

func TestUpdateUserRole_InvalidRole(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	_ = store.CreateUser("admin_user", "pass", "UTC", "admin")
	_ = store.CreateUser("target_user", "pass", "UTC", "viewer")
	admin, _ := store.Authenticate("admin_user", "pass")
	target, _ := store.Authenticate("target_user", "pass")

	userH := NewUserHandler(store)

	payload := map[string]string{"role": "superadmin"}
	body, _ := json.Marshal(payload)

	r := chi.NewRouter()
	r.Use(roleAndUserMiddleware(RoleAdmin, admin.ID))
	r.Patch("/api/users/{id}/role", userH.UpdateUserRole)

	req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/users/%d/role", target.ID), bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid role, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestDeleteUser_Works(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	_ = store.CreateUser("admin_user", "pass", "UTC", "admin")
	_ = store.CreateUser("to_delete", "pass", "UTC", "editor")
	admin, _ := store.Authenticate("admin_user", "pass")
	target, _ := store.Authenticate("to_delete", "pass")

	userH := NewUserHandler(store)

	r := chi.NewRouter()
	r.Use(roleAndUserMiddleware(RoleAdmin, admin.ID))
	r.Delete("/api/users/{id}", userH.DeleteUser)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/users/%d", target.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify user is gone
	_, err = store.GetUser(target.ID)
	if err == nil {
		t.Error("Expected error getting deleted user")
	}
}

func TestDeleteUser_RejectsSelfDeletion(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	_ = store.CreateUser("self_delete", "pass", "UTC", "admin")
	admin, _ := store.Authenticate("self_delete", "pass")

	userH := NewUserHandler(store)

	r := chi.NewRouter()
	r.Use(roleAndUserMiddleware(RoleAdmin, admin.ID))
	r.Delete("/api/users/{id}", userH.DeleteUser)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/users/%d", admin.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for self-deletion, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "cannot delete yourself" {
		t.Errorf("Expected error 'cannot delete yourself', got %s", resp["error"])
	}
}

func TestDeleteUser_PreventsDeleteLastAdmin(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	// Create only one admin and an editor
	_ = store.CreateUser("only_admin", "pass", "UTC", "admin")
	_ = store.CreateUser("requester", "pass", "UTC", "editor")
	onlyAdmin, _ := store.Authenticate("only_admin", "pass")
	requester, _ := store.Authenticate("requester", "pass")

	userH := NewUserHandler(store)

	r := chi.NewRouter()
	// Use requester context with admin role (simulating a scenario where this check matters)
	r.Use(roleAndUserMiddleware(RoleAdmin, requester.ID))
	r.Delete("/api/users/{id}", userH.DeleteUser)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/users/%d", onlyAdmin.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 when deleting last admin, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "cannot delete the last admin" {
		t.Errorf("Expected error 'cannot delete the last admin', got %s", resp["error"])
	}
}

func TestDeleteUser_NonAdminForbidden(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	_ = store.CreateUser("target", "pass", "UTC", "viewer")
	target, _ := store.Authenticate("target", "pass")

	userH := NewUserHandler(store)

	roles := []string{RoleEditor, RoleViewer}
	for _, role := range roles {
		t.Run(role, func(t *testing.T) {
			r := chi.NewRouter()
			r.Use(roleAndUserMiddleware(role, int64(9999)))
			r.Delete("/api/users/{id}", userH.DeleteUser)

			req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/users/%d", target.ID), nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("Expected 403 for role %s, got %d", role, w.Code)
			}
		})
	}
}
