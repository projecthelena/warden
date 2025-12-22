package db

import (
	"testing"
)

func TestAPIKeys(t *testing.T) {
	s := newTestStore(t)

	// Create
	key, err := s.CreateAPIKey("Test Key")
	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}
	if len(key) == 0 {
		t.Fatal("Returned key is empty")
	}

	// Validate Access
	valid, err := s.ValidateAPIKey(key)
	if err != nil {
		t.Fatalf("ValidateAPIKey failed: %v", err)
	}
	if !valid {
		t.Error("Expected key to be valid")
	}

	// Validate Fail
	valid, _ = s.ValidateAPIKey("sk_live_WRONG")
	if valid {
		t.Error("Expected invalid key to be rejected")
	}

	// List
	keys, err := s.ListAPIKeys()
	if err != nil {
		t.Fatalf("ListAPIKeys failed: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("Expected 1 key, got %d", len(keys))
	}

	// Delete
	if err := s.DeleteAPIKey(keys[0].ID); err != nil {
		t.Fatalf("DeleteAPIKey failed: %v", err)
	}

	// Verify Gone
	valid, _ = s.ValidateAPIKey(key)
	if valid {
		t.Error("Key should be invalid after deletion")
	}
}
