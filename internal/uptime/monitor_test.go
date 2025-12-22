package uptime

import (
	"testing"
	"time"
)

func TestMonitor_RecordResult(t *testing.T) {
	// 1. Initialize Monitor
	jobQueue := make(chan Job, 1)
	m := NewMonitor("m1", "g1", "Test Monitor", "http://example.com", 60*time.Second, jobQueue)

	// 2. Record 55 results (Limit is 50)
	for i := 0; i < 55; i++ {
		m.RecordResult(true, int64(100+i), time.Now(), 200, "", false)
	}

	// 3. Verify History Size
	history := m.GetHistory()
	if len(history) != 50 {
		t.Errorf("Expected history length 50, got %d", len(history))
	}

	// 4. Verify FIFO (Should contain last 50, so indices 5 to 54)
	// The last record recorded was i=54 (latency 154)
	last := history[len(history)-1]
	if last.Latency != 154 {
		t.Errorf("Expected last latency 154, got %d", last.Latency)
	}

	// The first record should be i=5 (latency 105)
	first := history[0]
	if first.Latency != 105 {
		t.Errorf("Expected first latency 105, got %d", first.Latency)
	}
}

func TestMonitor_GetLastStatus(t *testing.T) {
	jobQueue := make(chan Job, 1)
	m := NewMonitor("m1", "g1", "Test Monitor", "http://example.com", 60*time.Second, jobQueue)

	// Empty
	_, _, hasHistory, _ := m.GetLastStatus()
	if hasHistory {
		t.Error("Expected no history initially")
	}

	// Add record
	m.RecordResult(true, 200, time.Now(), 200, "", true)

	isUp, latency, hasHistory, isDegraded := m.GetLastStatus()
	if !hasHistory {
		t.Error("Expected history")
	}
	if !isUp {
		t.Error("Expected Up")
	}
	if latency != 200 {
		t.Errorf("Expected latency 200, got %d", latency)
	}
	if !isDegraded {
		t.Error("Expected degraded")
	}
}

func TestMonitor_Getters(t *testing.T) {
	jobQueue := make(chan Job, 1)
	m := NewMonitor("m1", "g1", "Monitor Name", "http://target.com", 10*time.Second, jobQueue)

	if m.GetName() != "Monitor Name" {
		t.Errorf("GetName incorrect")
	}
	if m.GetTargetURL() != "http://target.com" {
		t.Errorf("GetTargetURL incorrect")
	}
	if m.GetGroupID() != "g1" {
		t.Errorf("GetGroupID incorrect")
	}
	if m.GetInterval() != 10*time.Second {
		t.Errorf("GetInterval incorrect")
	}
}

func TestMonitor_Scheduling(t *testing.T) {
	jobQueue := make(chan Job, 10)
	// Short interval
	m := NewMonitor("m1", "g1", "Fast", "http://fast.com", 10*time.Millisecond, jobQueue)

	go m.Start()

	// Wait for at least 2 jobs
	timeout := time.After(100 * time.Millisecond)
	jobs := 0

loop:
	for {
		select {
		case <-jobQueue:
			jobs++
			if jobs >= 2 {
				break loop
			}
		case <-timeout:
			t.Fatal("Timeout waiting for scheduled jobs")
		}
	}

	m.Stop()

	// Ensure no panic on stop
	time.Sleep(20 * time.Millisecond)
}
