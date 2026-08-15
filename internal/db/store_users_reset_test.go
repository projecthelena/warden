package db

import (
	"errors"
	"testing"
	"time"
)

func TestSetUserPassword(t *testing.T) {
	RunTestWithBothDBs(t, "SetUserPassword", func(t *testing.T, s *Store) {
		if err := s.CreateUser("alice", "OldPass123!", "UTC", "admin"); err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		u, err := s.Authenticate("alice", "OldPass123!")
		if err != nil {
			t.Fatalf("Authenticate: %v", err)
		}
		if err := s.CreateSession(u.ID, "tok", time.Now().Add(time.Hour)); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}

		if err := s.SetUserPassword(u.ID, "FreshPass123!"); err != nil {
			t.Fatalf("SetUserPassword: %v", err)
		}
		if _, err := s.Authenticate("alice", "OldPass123!"); err == nil {
			t.Error("old password should stop working")
		}
		if _, err := s.Authenticate("alice", "FreshPass123!"); err != nil {
			t.Errorf("new password should work: %v", err)
		}
		if sess, _ := s.GetSession("tok"); sess != nil {
			t.Error("existing sessions should be revoked")
		}
	})
}

func TestSetUserPassword_Errors(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateUser("alice", "OldPass123!", "UTC", "admin")
	u, _ := s.Authenticate("alice", "OldPass123!")

	if err := s.SetUserPassword(9999, "FreshPass123!"); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("unknown id: want ErrUserNotFound, got %v", err)
	}
	// A password the login form would reject must never be stored.
	if err := s.SetUserPassword(u.ID, "short"); err == nil {
		t.Error("weak password should be rejected")
	}
	if _, err := s.Authenticate("alice", "OldPass123!"); err != nil {
		t.Error("a rejected reset must leave the old password intact")
	}
}

func TestResetPasswordByUsername(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateUser("alice", "OldPass123!", "UTC", "admin")

	// Username lookup is case-insensitive (CreateUser lower-cases on store).
	if err := s.ResetPasswordByUsername("ALICE", "FreshPass123!"); err != nil {
		t.Fatalf("ResetPasswordByUsername: %v", err)
	}
	if _, err := s.Authenticate("alice", "FreshPass123!"); err != nil {
		t.Errorf("new password should work: %v", err)
	}

	if err := s.ResetPasswordByUsername("ghost", "FreshPass123!"); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("unknown username: want ErrUserNotFound, got %v", err)
	}
}

func TestValidatePassword(t *testing.T) {
	ok := []string{"abc123!@", "P@ssw0rd", "12345678!9"}
	for _, p := range ok {
		if err := ValidatePassword(p); err != nil {
			t.Errorf("ValidatePassword(%q) = %v, want nil", p, err)
		}
	}
	bad := []string{"short1!", "noNumber!", "nonumber", "12345678", "onlyletters"}
	for _, p := range bad {
		if err := ValidatePassword(p); err == nil {
			t.Errorf("ValidatePassword(%q) = nil, want error", p)
		}
	}
}
