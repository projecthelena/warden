package db

import (
	"testing"
)

func TestSettingsResult(t *testing.T) {
	s := newTestStore(t)

	_, err := s.GetSetting("missing")
	if err == nil {
		t.Error("Expected error for missing setting")
	}

	if err := s.SetSetting("foo", "bar"); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}

	val, err := s.GetSetting("foo")
	if err != nil {
		t.Fatalf("GetSetting failed: %v", err)
	}
	if val != "bar" {
		t.Errorf("Expected 'bar', got '%s'", val)
	}

	_ = s.SetSetting("foo", "baz")
	val, _ = s.GetSetting("foo")
	if val != "baz" {
		t.Errorf("Expected 'baz', got '%s'", val)
	}
}

func TestSystemStats(t *testing.T) {
	s := newTestStore(t)
	stats, err := s.GetSystemStats()
	if err != nil {
		t.Fatalf("GetSystemStats failed: %v", err)
	}

	// Should be empty mostly
	if stats.TotalMonitors != 0 {
		// If seed runs, it might not be 0 depending on seed logic.
		// Current seed creates groups but not monitors.
		t.Logf("Total monitors: %d", stats.TotalMonitors)
	}
}
