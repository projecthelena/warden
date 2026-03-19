package db

import (
	"testing"
	"time"
)

func TestCreateUserWithRole(t *testing.T) {
	s := newTestStore(t)

	tests := []struct {
		username string
		role     string
	}{
		{"admin_user", "admin"},
		{"editor_user", "editor"},
		{"viewer_user", "viewer"},
		{"sv_user", "status_viewer"},
	}

	for _, tc := range tests {
		t.Run(tc.role, func(t *testing.T) {
			err := s.CreateUser(tc.username, "password123", "UTC", tc.role)
			if err != nil {
				t.Fatalf("CreateUser(%s, %s) failed: %v", tc.username, tc.role, err)
			}

			user, err := s.Authenticate(tc.username, "password123")
			if err != nil {
				t.Fatalf("Authenticate failed: %v", err)
			}
			if user.Role != tc.role {
				t.Errorf("Expected role %s, got %s", tc.role, user.Role)
			}
		})
	}
}

func TestCreateUserEmptyRoleDefaultsToAdmin(t *testing.T) {
	s := newTestStore(t)

	err := s.CreateUser("default_role_user", "password123", "UTC", "")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	user, err := s.Authenticate("default_role_user", "password123")
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if user.Role != "admin" {
		t.Errorf("Expected default role 'admin', got %s", user.Role)
	}
}

func TestGetUserRole(t *testing.T) {
	s := newTestStore(t)

	err := s.CreateUser("role_test", "password123", "UTC", "editor")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	user, err := s.Authenticate("role_test", "password123")
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	role, err := s.GetUserRole(user.ID)
	if err != nil {
		t.Fatalf("GetUserRole failed: %v", err)
	}
	if role != "editor" {
		t.Errorf("Expected role 'editor', got %s", role)
	}
}

func TestGetUserRole_NotFound(t *testing.T) {
	s := newTestStore(t)

	_, err := s.GetUserRole(99999)
	if err != ErrUserNotFound {
		t.Errorf("Expected ErrUserNotFound, got %v", err)
	}
}

func TestListUsersWithRoles(t *testing.T) {
	s := newTestStore(t)

	// Create users with different roles
	err := s.CreateUser("admin1", "pass", "UTC", "admin")
	if err != nil {
		t.Fatalf("CreateUser admin1 failed: %v", err)
	}
	err = s.CreateUser("editor1", "pass", "UTC", "editor")
	if err != nil {
		t.Fatalf("CreateUser editor1 failed: %v", err)
	}
	err = s.CreateUser("viewer1", "pass", "UTC", "viewer")
	if err != nil {
		t.Fatalf("CreateUser viewer1 failed: %v", err)
	}
	err = s.CreateUser("sv1", "pass", "UTC", "status_viewer")
	if err != nil {
		t.Fatalf("CreateUser sv1 failed: %v", err)
	}

	users, err := s.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if len(users) != 4 {
		t.Fatalf("Expected 4 users, got %d", len(users))
	}

	// Build a map for easy lookup
	roleMap := make(map[string]string)
	for _, u := range users {
		roleMap[u.Username] = u.Role
	}

	expected := map[string]string{
		"admin1":  "admin",
		"editor1": "editor",
		"viewer1": "viewer",
		"sv1":     "status_viewer",
	}
	for username, expectedRole := range expected {
		if got, ok := roleMap[username]; !ok {
			t.Errorf("User %s not found in list", username)
		} else if got != expectedRole {
			t.Errorf("User %s: expected role %s, got %s", username, expectedRole, got)
		}
	}
}

func TestUpdateUserRole(t *testing.T) {
	s := newTestStore(t)

	err := s.CreateUser("role_change", "pass", "UTC", "viewer")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	user, err := s.Authenticate("role_change", "pass")
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	// Verify initial role
	role, err := s.GetUserRole(user.ID)
	if err != nil {
		t.Fatalf("GetUserRole failed: %v", err)
	}
	if role != "viewer" {
		t.Errorf("Expected initial role 'viewer', got %s", role)
	}

	// Update to editor
	err = s.UpdateUserRole(user.ID, "editor")
	if err != nil {
		t.Fatalf("UpdateUserRole failed: %v", err)
	}

	role, err = s.GetUserRole(user.ID)
	if err != nil {
		t.Fatalf("GetUserRole after update failed: %v", err)
	}
	if role != "editor" {
		t.Errorf("Expected role 'editor' after update, got %s", role)
	}

	// Update to admin
	err = s.UpdateUserRole(user.ID, "admin")
	if err != nil {
		t.Fatalf("UpdateUserRole to admin failed: %v", err)
	}

	role, err = s.GetUserRole(user.ID)
	if err != nil {
		t.Fatalf("GetUserRole after second update failed: %v", err)
	}
	if role != "admin" {
		t.Errorf("Expected role 'admin' after update, got %s", role)
	}

	// Also verify via GetUser
	u, err := s.GetUser(user.ID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if u.Role != "admin" {
		t.Errorf("GetUser: expected role 'admin', got %s", u.Role)
	}
}

func TestDeleteUser(t *testing.T) {
	s := newTestStore(t)

	err := s.CreateUser("to_delete", "pass", "UTC", "editor")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	user, err := s.Authenticate("to_delete", "pass")
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	// Create a session for the user
	token := "delete-test-token"
	expires := time.Now().Add(1 * time.Hour)
	if err := s.CreateSession(user.ID, token, expires); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Verify session exists
	sess, err := s.GetSession(token)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if sess == nil {
		t.Fatal("Expected session to exist before delete")
	}

	// Delete the user
	err = s.DeleteUser(user.ID)
	if err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}

	// Verify user is gone
	_, err = s.GetUser(user.ID)
	if err == nil {
		t.Error("Expected error getting deleted user, got nil")
	}

	// Verify sessions are cleaned up
	sess, err = s.GetSession(token)
	if err != nil {
		t.Fatalf("GetSession after delete failed: %v", err)
	}
	if sess != nil {
		t.Error("Expected session to be deleted with user")
	}
}

func TestCountAdmins(t *testing.T) {
	s := newTestStore(t)

	// No users yet
	count, err := s.CountAdmins()
	if err != nil {
		t.Fatalf("CountAdmins failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 admins, got %d", count)
	}

	// Create one admin
	err = s.CreateUser("admin1", "pass", "UTC", "admin")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	count, err = s.CountAdmins()
	if err != nil {
		t.Fatalf("CountAdmins failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 admin, got %d", count)
	}

	// Create another admin
	err = s.CreateUser("admin2", "pass", "UTC", "admin")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	count, err = s.CountAdmins()
	if err != nil {
		t.Fatalf("CountAdmins failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 admins, got %d", count)
	}

	// Create non-admin users - count should not change
	err = s.CreateUser("editor1", "pass", "UTC", "editor")
	if err != nil {
		t.Fatalf("CreateUser editor failed: %v", err)
	}
	err = s.CreateUser("viewer1", "pass", "UTC", "viewer")
	if err != nil {
		t.Fatalf("CreateUser viewer failed: %v", err)
	}
	err = s.CreateUser("sv1", "pass", "UTC", "status_viewer")
	if err != nil {
		t.Fatalf("CreateUser status_viewer failed: %v", err)
	}

	count, err = s.CountAdmins()
	if err != nil {
		t.Fatalf("CountAdmins failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 admins (non-admin users should not be counted), got %d", count)
	}
}

func TestUserStatusPages(t *testing.T) {
	s := newTestStore(t)

	// Create a user
	err := s.CreateUser("sp_user", "pass", "UTC", "status_viewer")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	user, err := s.Authenticate("sp_user", "pass")
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	// Initially no status pages assigned
	ids, err := s.GetUserStatusPages(user.ID)
	if err != nil {
		t.Fatalf("GetUserStatusPages failed: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("Expected 0 status pages initially, got %d", len(ids))
	}

	// Create status pages for assignment
	err = s.UpsertStatusPageFull(StatusPageInput{
		Slug:    "test-page-1",
		Title:   "Test Page 1",
		Enabled: true,
		Theme:   "system",
	})
	if err != nil {
		t.Fatalf("UpsertStatusPageFull 1 failed: %v", err)
	}
	err = s.UpsertStatusPageFull(StatusPageInput{
		Slug:    "test-page-2",
		Title:   "Test Page 2",
		Enabled: true,
		Theme:   "system",
	})
	if err != nil {
		t.Fatalf("UpsertStatusPageFull 2 failed: %v", err)
	}

	sp1, err := s.GetStatusPageBySlug("test-page-1")
	if err != nil {
		t.Fatalf("GetStatusPageBySlug 1 failed: %v", err)
	}
	if sp1 == nil {
		t.Fatal("Status page 1 not found")
	}

	sp2, err := s.GetStatusPageBySlug("test-page-2")
	if err != nil {
		t.Fatalf("GetStatusPageBySlug 2 failed: %v", err)
	}
	if sp2 == nil {
		t.Fatal("Status page 2 not found")
	}

	// Assign both status pages
	err = s.SetUserStatusPages(user.ID, []int64{sp1.ID, sp2.ID})
	if err != nil {
		t.Fatalf("SetUserStatusPages failed: %v", err)
	}

	// Verify assignment
	ids, err = s.GetUserStatusPages(user.ID)
	if err != nil {
		t.Fatalf("GetUserStatusPages failed: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("Expected 2 status pages, got %d", len(ids))
	}

	// Verify the IDs match (order may vary)
	idSet := make(map[int64]bool)
	for _, id := range ids {
		idSet[id] = true
	}
	if !idSet[sp1.ID] {
		t.Errorf("Expected status page ID %d in assignments", sp1.ID)
	}
	if !idSet[sp2.ID] {
		t.Errorf("Expected status page ID %d in assignments", sp2.ID)
	}

	// Replace with just one status page
	err = s.SetUserStatusPages(user.ID, []int64{sp1.ID})
	if err != nil {
		t.Fatalf("SetUserStatusPages (replace) failed: %v", err)
	}

	ids, err = s.GetUserStatusPages(user.ID)
	if err != nil {
		t.Fatalf("GetUserStatusPages after replace failed: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("Expected 1 status page after replace, got %d", len(ids))
	}
	if ids[0] != sp1.ID {
		t.Errorf("Expected status page ID %d, got %d", sp1.ID, ids[0])
	}

	// Clear all assignments
	err = s.SetUserStatusPages(user.ID, []int64{})
	if err != nil {
		t.Fatalf("SetUserStatusPages (clear) failed: %v", err)
	}

	ids, err = s.GetUserStatusPages(user.ID)
	if err != nil {
		t.Fatalf("GetUserStatusPages after clear failed: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("Expected 0 status pages after clear, got %d", len(ids))
	}
}

func TestFindOrCreateSSOUser_DefaultViewerRole(t *testing.T) {
	s := newTestStore(t)

	user, err := s.FindOrCreateSSOUser("google", "sso-id-123", "sso@example.com", "SSO User", "https://example.com/avatar.jpg", true)
	if err != nil {
		t.Fatalf("FindOrCreateSSOUser failed: %v", err)
	}

	if user.Role != "viewer" {
		t.Errorf("Expected new SSO user to have role 'viewer', got %s", user.Role)
	}

	// Verify via GetUser
	fetched, err := s.GetUser(user.ID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if fetched.Role != "viewer" {
		t.Errorf("GetUser: expected role 'viewer', got %s", fetched.Role)
	}

	// Verify via GetUserRole
	role, err := s.GetUserRole(user.ID)
	if err != nil {
		t.Fatalf("GetUserRole failed: %v", err)
	}
	if role != "viewer" {
		t.Errorf("GetUserRole: expected 'viewer', got %s", role)
	}
}

func TestFindOrCreateSSOUser_ExistingUserKeepsRole(t *testing.T) {
	s := newTestStore(t)

	// Create SSO user first time (gets viewer role)
	user, err := s.FindOrCreateSSOUser("google", "sso-id-456", "keep@example.com", "Keeper", "", true)
	if err != nil {
		t.Fatalf("FindOrCreateSSOUser (first) failed: %v", err)
	}
	if user.Role != "viewer" {
		t.Fatalf("Expected initial role 'viewer', got %s", user.Role)
	}

	// Promote to admin
	err = s.UpdateUserRole(user.ID, "admin")
	if err != nil {
		t.Fatalf("UpdateUserRole failed: %v", err)
	}

	// Login again via SSO - should keep admin role
	user2, err := s.FindOrCreateSSOUser("google", "sso-id-456", "keep@example.com", "Keeper", "", true)
	if err != nil {
		t.Fatalf("FindOrCreateSSOUser (second) failed: %v", err)
	}
	if user2.Role != "admin" {
		t.Errorf("Expected existing SSO user to keep role 'admin', got %s", user2.Role)
	}
}

func TestAPIKeyWithRole(t *testing.T) {
	s := newTestStore(t)

	// Create API key with editor role
	key, err := s.CreateAPIKey("Editor Key", "editor")
	if err != nil {
		t.Fatalf("CreateAPIKey (editor) failed: %v", err)
	}

	valid, role, err := s.ValidateAPIKey(key)
	if err != nil {
		t.Fatalf("ValidateAPIKey failed: %v", err)
	}
	if !valid {
		t.Fatal("Expected key to be valid")
	}
	if role != "editor" {
		t.Errorf("Expected role 'editor', got %s", role)
	}

	// Create API key with viewer role
	key2, err := s.CreateAPIKey("Viewer Key", "viewer")
	if err != nil {
		t.Fatalf("CreateAPIKey (viewer) failed: %v", err)
	}

	valid, role, err = s.ValidateAPIKey(key2)
	if err != nil {
		t.Fatalf("ValidateAPIKey (viewer) failed: %v", err)
	}
	if !valid {
		t.Fatal("Expected key to be valid")
	}
	if role != "viewer" {
		t.Errorf("Expected role 'viewer', got %s", role)
	}

	// Create API key with admin role
	key3, err := s.CreateAPIKey("Admin Key", "admin")
	if err != nil {
		t.Fatalf("CreateAPIKey (admin) failed: %v", err)
	}

	valid, role, err = s.ValidateAPIKey(key3)
	if err != nil {
		t.Fatalf("ValidateAPIKey (admin) failed: %v", err)
	}
	if !valid {
		t.Fatal("Expected key to be valid")
	}
	if role != "admin" {
		t.Errorf("Expected role 'admin', got %s", role)
	}
}

func TestAPIKeyDefaultRole(t *testing.T) {
	s := newTestStore(t)

	// Create API key with empty role - should default to editor
	key, err := s.CreateAPIKey("Default Role Key", "")
	if err != nil {
		t.Fatalf("CreateAPIKey (empty) failed: %v", err)
	}

	valid, role, err := s.ValidateAPIKey(key)
	if err != nil {
		t.Fatalf("ValidateAPIKey failed: %v", err)
	}
	if !valid {
		t.Fatal("Expected key to be valid")
	}
	if role != "editor" {
		t.Errorf("Expected default role 'editor', got %s", role)
	}
}

func TestAPIKeyInvalidKey(t *testing.T) {
	s := newTestStore(t)

	// Short key
	valid, role, err := s.ValidateAPIKey("short")
	if err != nil {
		t.Fatalf("ValidateAPIKey (short) failed: %v", err)
	}
	if valid {
		t.Error("Expected short key to be invalid")
	}
	if role != "" {
		t.Errorf("Expected empty role for invalid key, got %s", role)
	}

	// Non-existent key with proper prefix length
	valid, role, err = s.ValidateAPIKey("sk_live_0000deadbeef0000")
	if err != nil {
		t.Fatalf("ValidateAPIKey (nonexistent) failed: %v", err)
	}
	if valid {
		t.Error("Expected nonexistent key to be invalid")
	}
	if role != "" {
		t.Errorf("Expected empty role for invalid key, got %s", role)
	}
}
