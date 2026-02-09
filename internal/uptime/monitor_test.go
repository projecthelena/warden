package uptime

import (
	"testing"
	"time"
)

func TestMonitor_RecordResult(t *testing.T) {
	// 1. Initialize Monitor
	jobQueue := make(chan Job, 1)
	m := NewMonitor("m1", "g1", "Test Monitor", "http://example.com", 60*time.Second, jobQueue, time.Now())

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
	m := NewMonitor("m1", "g1", "Test Monitor", "http://example.com", 60*time.Second, jobQueue, time.Now())

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
	m := NewMonitor("m1", "g1", "Monitor Name", "http://target.com", 10*time.Second, jobQueue, time.Now())

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
	m := NewMonitor("m1", "g1", "Fast", "http://fast.com", 10*time.Millisecond, jobQueue, time.Now())

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

func TestAlignDelay(t *testing.T) {
	interval := 60 * time.Second

	tests := []struct {
		name      string
		createdAt time.Time
		now       time.Time
		want      time.Duration
	}{
		{
			name:      "exactly on aligned tick",
			createdAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			now:       time.Date(2025, 1, 1, 0, 3, 0, 0, time.UTC), // 3 intervals later
			want:      0,
		},
		{
			name:      "half interval elapsed",
			createdAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			now:       time.Date(2025, 1, 1, 0, 2, 30, 0, time.UTC), // 2.5 intervals
			want:      30 * time.Second,
		},
		{
			name:      "quarter interval elapsed",
			createdAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			now:       time.Date(2025, 1, 1, 0, 0, 15, 0, time.UTC), // 0.25 intervals
			want:      45 * time.Second,
		},
		{
			name:      "just after creation",
			createdAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			now:       time.Date(2025, 1, 1, 0, 0, 1, 0, time.UTC), // 1 second after
			want:      59 * time.Second,
		},
		{
			name:      "now equals createdAt",
			createdAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			now:       time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			want:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alignDelay(tt.createdAt, interval, tt.now)
			if got != tt.want {
				t.Errorf("alignDelay() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAlignDelay_DifferentIntervals(t *testing.T) {
	createdAt := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// 30-second interval, 45 seconds elapsed → 15s until next tick
	got := alignDelay(createdAt, 30*time.Second, createdAt.Add(45*time.Second))
	if got != 15*time.Second {
		t.Errorf("30s interval: got %v, want 15s", got)
	}

	// 5-minute interval, 7.5 minutes elapsed → 2.5 minutes until next tick
	got = alignDelay(createdAt, 5*time.Minute, createdAt.Add(7*time.Minute+30*time.Second))
	if got != 2*time.Minute+30*time.Second {
		t.Errorf("5m interval: got %v, want 2m30s", got)
	}
}

func TestMonitor_AlignedScheduling(t *testing.T) {
	jobQueue := make(chan Job, 20)
	interval := 50 * time.Millisecond

	// Create monitor with createdAt in the past, aligned so next tick is ~25ms from now
	// This tests that the monitor actually uses aligned scheduling
	createdAt := time.Now().Add(-interval - interval/2) // 1.5 intervals ago → next tick in 0.5 interval

	m := NewMonitor("m1", "g1", "Aligned", "http://aligned.com", interval, jobQueue, createdAt)

	go m.Start()

	// Wait for at least 3 jobs (initial + aligned + ticker)
	timeout := time.After(500 * time.Millisecond)
	jobs := 0

loop:
	for {
		select {
		case <-jobQueue:
			jobs++
			if jobs >= 3 {
				break loop
			}
		case <-timeout:
			t.Fatalf("Timeout waiting for aligned scheduled jobs, got %d", jobs)
		}
	}

	m.Stop()
	time.Sleep(20 * time.Millisecond)
}

func TestNewMonitor_ZeroCreatedAtDefaultsToNow(t *testing.T) {
	jobQueue := make(chan Job, 1)
	before := time.Now()
	m := NewMonitor("m1", "g1", "Zero", "http://zero.com", 60*time.Second, jobQueue, time.Time{})
	after := time.Now()

	if m.createdAt.Before(before) || m.createdAt.After(after) {
		t.Errorf("Expected createdAt to be set to ~now, got %v", m.createdAt)
	}
}
