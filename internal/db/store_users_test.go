package db

import (
	"testing"
	"time"
)

func TestUserLifecycle(t *testing.T) {
	s := newTestStore(t)

	// 1. Create User
	err := s.CreateUser("admin", "secret123", "UTC")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// 2. Authenticate Success
	user, err := s.Authenticate("admin", "secret123")
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if user.Username != "admin" {
		t.Errorf("Expected username admin, got %s", user.Username)
	}
	if user.ID == 0 {
		t.Error("Expected valid ID > 0")
	}

	// 3. Authenticate Failure
	_, err = s.Authenticate("admin", "wrongpass")
	if err != ErrInvalidPass {
		t.Errorf("Expected ErrInvalidPass, got %v", err)
	}
	_, err = s.Authenticate("nonexistent", "secret123")
	if err != ErrUserNotFound {
		t.Errorf("Expected ErrUserNotFound, got %v", err)
	}

	// 3b. Strict Login Verification (Case Sensitive)
	_, err = s.Authenticate("Admin", "secret123")
	if err != ErrUserNotFound {
		t.Errorf("Expected ErrUserNotFound for 'Admin' (Strict Mode), got %v", err)
	}

	// 4. Update User
	if err := s.UpdateUser(user.ID, "newpass456", "EST"); err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}

	// Verify Update
	u2, err := s.GetUser(user.ID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if u2.Timezone != "EST" {
		t.Errorf("Expected timezone EST, got %s", u2.Timezone)
	}

	// Authenticate with new pass
	_, err = s.Authenticate("admin", "newpass456")
	if err != nil {
		t.Errorf("Authenticate with new pass failed: %v", err)
	}
}

func TestSessions(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateUser("user1", "pass", "UTC")
	u, _ := s.Authenticate("user1", "pass")

	// Create Session
	token := "abc-123-token"
	expires := time.Now().Add(1 * time.Hour)
	if err := s.CreateSession(u.ID, token, expires); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Get Session
	sess, err := s.GetSession(token)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if sess.UserID != u.ID {
		t.Errorf("UserID mismatch")
	}

	// Delete Session
	if err := s.DeleteSession(token); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	// Verify Gone
	sess2, _ := s.GetSession(token)
	if sess2 != nil {
		t.Error("Session should be gone")
	}
}

func TestHasUsers(t *testing.T) {
	s := newTestStore(t)

	has, err := s.HasUsers()
	if err != nil {
		t.Fatalf("HasUsers failed: %v", err)
	}
	if has {
		t.Error("Expected no users initially")
	}

	_ = s.CreateUser("u", "p", "UTC")

	has, _ = s.HasUsers()
	if !has {
		t.Error("Expected users after creation")
	}
}
