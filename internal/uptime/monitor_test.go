package uptime

import (
	"sync"
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

// --- Notification Fatigue Edge Case Tests ---

func newTestMonitorWithConfig(cfg MonitorConfig) *Monitor {
	jq := make(chan Job, 1)
	m := NewMonitor("test", "g1", "Test", "http://example.com", 60*time.Second, jq, time.Now())
	m.ApplyConfig(cfg)
	return m
}

func TestMonitor_ConfirmationChecks(t *testing.T) {
	t.Run("threshold_1_immediate", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 1, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		confirmed := m.IncrementDown()
		if !confirmed {
			t.Error("Expected IncrementDown to return true with threshold=1")
		}
		if !m.IsConfirmedDown() {
			t.Error("Expected IsConfirmedDown=true")
		}
	})

	t.Run("N_minus_1_no_confirm", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		m.IncrementDown()
		confirmed := m.IncrementDown()
		if confirmed {
			t.Error("Expected false after 2 of 3 increments")
		}
		if m.IsConfirmedDown() {
			t.Error("Expected IsConfirmedDown=false after 2 of 3")
		}
	})

	t.Run("exactly_N_confirms", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		r1 := m.IncrementDown()
		r2 := m.IncrementDown()
		r3 := m.IncrementDown()
		if r1 || r2 {
			t.Error("Expected false for first two increments")
		}
		if !r3 {
			t.Error("Expected true on third increment")
		}
		if !m.IsConfirmedDown() {
			t.Error("Expected IsConfirmedDown=true")
		}
	})

	t.Run("reset_resets_counter", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		m.IncrementDown()
		m.IncrementDown()
		m.IncrementDown() // confirmed
		wasConfirmed := m.ResetDown()
		if !wasConfirmed {
			t.Error("Expected ResetDown to return true (was confirmed)")
		}
		if m.IsConfirmedDown() {
			t.Error("Expected IsConfirmedDown=false after reset")
		}
		// 2 more increments should not confirm (need 3)
		m.IncrementDown()
		m.IncrementDown()
		if m.IsConfirmedDown() {
			t.Error("Expected IsConfirmedDown=false after 2 increments post-reset")
		}
	})

	t.Run("increment_after_confirmed_returns_false", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		m.IncrementDown()
		m.IncrementDown()
		m.IncrementDown() // confirmed
		r4 := m.IncrementDown()
		if r4 {
			t.Error("Expected false after already confirmed")
		}
	})

	t.Run("reset_when_never_confirmed", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		wasConfirmed := m.ResetDown()
		if wasConfirmed {
			t.Error("Expected ResetDown to return false (never confirmed)")
		}
	})

	t.Run("interleaved_down_degraded", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		m.IncrementDegraded()
		m.IncrementDegraded()
		m.ResetDegraded()
		m.IncrementDown()
		// Degraded counter should be gone after reset
		if m.IsConfirmedDegraded() {
			t.Error("Expected IsConfirmedDegraded=false after ResetDegraded")
		}
	})

	t.Run("config_change_lowers_threshold", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		m.IncrementDown()
		m.IncrementDown()
		// Now lower threshold to 2
		m.ApplyConfig(MonitorConfig{ConfirmationThreshold: 2, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		// Next increment should confirm (count=3 >= threshold=2)
		confirmed := m.IncrementDown()
		if !confirmed {
			t.Error("Expected confirmation after lowering threshold")
		}
	})

	t.Run("config_change_raises_threshold", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		m.IncrementDown()
		m.IncrementDown()
		// Raise threshold to 5
		m.ApplyConfig(MonitorConfig{ConfirmationThreshold: 5, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		confirmed := m.IncrementDown()
		if confirmed {
			t.Error("Expected no confirmation after raising threshold (count=3 < 5)")
		}
		if m.IsConfirmedDown() {
			t.Error("Expected IsConfirmedDown=false")
		}
	})

	t.Run("degraded_mirrors_down", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		r1 := m.IncrementDegraded()
		r2 := m.IncrementDegraded()
		r3 := m.IncrementDegraded()
		if r1 || r2 {
			t.Error("Expected false for first two degraded increments")
		}
		if !r3 {
			t.Error("Expected true on third degraded increment")
		}
		if !m.IsConfirmedDegraded() {
			t.Error("Expected IsConfirmedDegraded=true")
		}
		wasConfirmed := m.ResetDegraded()
		if !wasConfirmed {
			t.Error("Expected ResetDegraded to return true")
		}
		if m.IsConfirmedDegraded() {
			t.Error("Expected IsConfirmedDegraded=false after reset")
		}
	})
}

func TestMonitor_HydrateConfirmationState(t *testing.T) {
	t.Run("empty_history", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		m.HydrateConfirmationState()
		if m.IsConfirmedDown() {
			t.Error("Expected not confirmed with empty history")
		}
		if m.IsConfirmedDegraded() {
			t.Error("Expected not confirmed degraded with empty history")
		}
	})

	t.Run("exactly_threshold_failures", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		now := time.Now()
		m.RecordResult(false, 0, now.Add(-3*time.Minute), 0, "err", false)
		m.RecordResult(false, 0, now.Add(-2*time.Minute), 0, "err", false)
		m.RecordResult(false, 0, now.Add(-1*time.Minute), 0, "err", false)
		m.HydrateConfirmationState()
		if !m.IsConfirmedDown() {
			t.Error("Expected IsConfirmedDown=true with 3 consecutive failures")
		}
	})

	t.Run("threshold_minus_1", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		now := time.Now()
		m.RecordResult(false, 0, now.Add(-2*time.Minute), 0, "err", false)
		m.RecordResult(false, 0, now.Add(-1*time.Minute), 0, "err", false)
		m.HydrateConfirmationState()
		if m.IsConfirmedDown() {
			t.Error("Expected IsConfirmedDown=false with 2 failures (threshold=3)")
		}
	})

	t.Run("non_consecutive_failures", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		now := time.Now()
		m.RecordResult(false, 0, now.Add(-4*time.Minute), 0, "err", false)
		m.RecordResult(false, 0, now.Add(-3*time.Minute), 0, "err", false)
		m.RecordResult(true, 100, now.Add(-2*time.Minute), 200, "", false)
		m.RecordResult(false, 0, now.Add(-1*time.Minute), 0, "err", false)
		m.HydrateConfirmationState()
		// Trailing consecutive failures = 1
		if m.IsConfirmedDown() {
			t.Error("Expected IsConfirmedDown=false (trailing consecutive=1)")
		}
	})

	t.Run("degraded_hydration", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		now := time.Now()
		m.RecordResult(true, 5000, now.Add(-3*time.Minute), 200, "", true)
		m.RecordResult(true, 5000, now.Add(-2*time.Minute), 200, "", true)
		m.RecordResult(true, 5000, now.Add(-1*time.Minute), 200, "", true)
		m.HydrateConfirmationState()
		if !m.IsConfirmedDegraded() {
			t.Error("Expected IsConfirmedDegraded=true with 3 consecutive degraded")
		}
	})

	t.Run("down_priority_over_degraded", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		now := time.Now()
		m.RecordResult(true, 5000, now.Add(-5*time.Minute), 200, "", true)  // degraded
		m.RecordResult(true, 5000, now.Add(-4*time.Minute), 200, "", true)  // degraded
		m.RecordResult(false, 0, now.Add(-3*time.Minute), 0, "err", false)  // down
		m.RecordResult(false, 0, now.Add(-2*time.Minute), 0, "err", false)  // down
		m.RecordResult(false, 0, now.Add(-1*time.Minute), 0, "err", false)  // down
		m.HydrateConfirmationState()
		if !m.IsConfirmedDown() {
			t.Error("Expected IsConfirmedDown=true (3 consecutive down)")
		}
		// Degraded should NOT be confirmed because down has priority
		if m.IsConfirmedDegraded() {
			t.Error("Expected IsConfirmedDegraded=false (down takes priority)")
		}
	})

	t.Run("mixed_ending_up", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		now := time.Now()
		m.RecordResult(false, 0, now.Add(-4*time.Minute), 0, "err", false)
		m.RecordResult(false, 0, now.Add(-3*time.Minute), 0, "err", false)
		m.RecordResult(false, 0, now.Add(-2*time.Minute), 0, "err", false)
		m.RecordResult(true, 100, now.Add(-1*time.Minute), 200, "", false) // last is up
		m.HydrateConfirmationState()
		if m.IsConfirmedDown() {
			t.Error("Expected IsConfirmedDown=false (last check is up)")
		}
	})
}

func TestMonitor_Cooldown(t *testing.T) {
	t.Run("zero_always_notifies", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 0, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		m.MarkNotified("down")
		if !m.ShouldNotify("down") {
			t.Error("Expected ShouldNotify=true with cooldown=0")
		}
	})

	t.Run("active_suppresses", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		m.MarkNotified("down")
		if m.ShouldNotify("down") {
			t.Error("Expected ShouldNotify=false within cooldown")
		}
	})

	t.Run("expired_allows", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 1, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		// Manually set lastNotifiedAt to 2 minutes ago
		m.mu.Lock()
		m.lastNotifiedAt["down"] = time.Now().Add(-2 * time.Minute)
		m.mu.Unlock()
		if !m.ShouldNotify("down") {
			t.Error("Expected ShouldNotify=true after cooldown expired")
		}
	})

	t.Run("per_event_type_independent", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		m.MarkNotified("down")
		if !m.ShouldNotify("degraded") {
			t.Error("Expected ShouldNotify('degraded')=true when only 'down' was notified")
		}
	})

	t.Run("nil_map_defensive", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		m.mu.Lock()
		m.lastNotifiedAt = nil
		m.mu.Unlock()
		// MarkNotified should not panic — it initializes the map
		m.MarkNotified("down")
		if m.ShouldNotify("down") {
			t.Error("Expected ShouldNotify=false after MarkNotified")
		}
	})

	t.Run("reset_down_clears_cooldown", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		// Confirm down then mark notified
		m.IncrementDown()
		m.IncrementDown()
		m.IncrementDown()
		m.MarkNotified("down")
		if m.ShouldNotify("down") {
			t.Error("Expected suppressed during cooldown")
		}
		m.ResetDown()
		if !m.ShouldNotify("down") {
			t.Error("Expected ShouldNotify=true after ResetDown (cooldown cleared)")
		}
	})

	t.Run("reset_degraded_clears_cooldown", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		m.IncrementDegraded()
		m.IncrementDegraded()
		m.IncrementDegraded()
		m.MarkNotified("degraded")
		if m.ShouldNotify("degraded") {
			t.Error("Expected suppressed during cooldown")
		}
		m.ResetDegraded()
		if !m.ShouldNotify("degraded") {
			t.Error("Expected ShouldNotify=true after ResetDegraded (cooldown cleared)")
		}
	})

	t.Run("reset_unconfirmed_keeps_cooldown", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		m.MarkNotified("down")
		// Never confirmed, so ResetDown won't clear cooldown
		m.ResetDown()
		if m.ShouldNotify("down") {
			t.Error("Expected ShouldNotify=false (cooldown not cleared because never confirmed)")
		}
	})
}

func TestMonitor_FlapDetection(t *testing.T) {
	defaultCfg := MonitorConfig{
		ConfirmationThreshold: 3,
		CooldownMinutes:       30,
		FlapDetectionEnabled:  true,
		FlapWindowChecks:      21,
		FlapThresholdPercent:  25,
	}

	t.Run("empty_history", func(t *testing.T) {
		m := newTestMonitorWithConfig(defaultCfg)
		isFlapping, changed := m.ComputeFlapping()
		if isFlapping || changed {
			t.Errorf("Expected (false, false) with empty history, got (%v, %v)", isFlapping, changed)
		}
	})

	t.Run("two_items_insufficient", func(t *testing.T) {
		m := newTestMonitorWithConfig(defaultCfg)
		now := time.Now()
		m.RecordResult(true, 100, now.Add(-2*time.Minute), 200, "", false)
		m.RecordResult(false, 0, now.Add(-1*time.Minute), 0, "err", false)
		isFlapping, changed := m.ComputeFlapping()
		if isFlapping || changed {
			t.Errorf("Expected (false, false) with <3 items, got (%v, %v)", isFlapping, changed)
		}
	})

	t.Run("no_transitions", func(t *testing.T) {
		m := newTestMonitorWithConfig(defaultCfg)
		now := time.Now()
		for i := 0; i < 10; i++ {
			m.RecordResult(true, 100, now.Add(time.Duration(-10+i)*time.Minute), 200, "", false)
		}
		isFlapping, _ := m.ComputeFlapping()
		if isFlapping {
			t.Error("Expected not flapping with all-up history")
		}
	})

	t.Run("all_transitions", func(t *testing.T) {
		m := newTestMonitorWithConfig(defaultCfg)
		now := time.Now()
		for i := 0; i < 10; i++ {
			isUp := i%2 == 0
			m.RecordResult(isUp, 100, now.Add(time.Duration(-10+i)*time.Minute), 200, "", false)
		}
		isFlapping, changed := m.ComputeFlapping()
		if !isFlapping {
			t.Error("Expected flapping with 100% transitions")
		}
		if !changed {
			t.Error("Expected changed=true on first flapping detection")
		}
	})

	t.Run("at_threshold", func(t *testing.T) {
		// With window=20, we need exactly 25% transitions (5 out of 19 possible)
		cfg := MonitorConfig{
			ConfirmationThreshold: 3,
			CooldownMinutes:       30,
			FlapDetectionEnabled:  true,
			FlapWindowChecks:      20,
			FlapThresholdPercent:  25,
		}
		m := newTestMonitorWithConfig(cfg)
		now := time.Now()
		// Build history: 15 "up", then alternate 5 times to get transitions
		for i := 0; i < 15; i++ {
			m.RecordResult(true, 100, now.Add(time.Duration(-20+i)*time.Minute), 200, "", false)
		}
		// Now 5 alternating results → 5 transitions in last 20 checks → 5/19 = 26% > 25%
		for i := 0; i < 5; i++ {
			isUp := i%2 != 0
			m.RecordResult(isUp, 100, now.Add(time.Duration(-5+i)*time.Minute), 200, "", false)
		}
		isFlapping, _ := m.ComputeFlapping()
		if !isFlapping {
			t.Error("Expected flapping at threshold")
		}
	})

	t.Run("below_threshold", func(t *testing.T) {
		// Ensure transitions are well below 25%
		cfg := MonitorConfig{
			ConfirmationThreshold: 3,
			CooldownMinutes:       30,
			FlapDetectionEnabled:  true,
			FlapWindowChecks:      21,
			FlapThresholdPercent:  25,
		}
		m := newTestMonitorWithConfig(cfg)
		now := time.Now()
		// 19 up, 2 down at end — transitions: 1 (up→down) out of 20 = 5%
		for i := 0; i < 19; i++ {
			m.RecordResult(true, 100, now.Add(time.Duration(-21+i)*time.Minute), 200, "", false)
		}
		m.RecordResult(false, 0, now.Add(-2*time.Minute), 0, "err", false)
		m.RecordResult(false, 0, now.Add(-1*time.Minute), 0, "err", false)
		isFlapping, _ := m.ComputeFlapping()
		if isFlapping {
			t.Error("Expected not flapping (well below threshold)")
		}
	})

	t.Run("hysteresis_stays_flapping", func(t *testing.T) {
		cfg := MonitorConfig{
			ConfirmationThreshold: 3,
			CooldownMinutes:       30,
			FlapDetectionEnabled:  true,
			FlapWindowChecks:      10,
			FlapThresholdPercent:  25, // stop threshold = 25*80/100 = 20
		}
		m := newTestMonitorWithConfig(cfg)
		now := time.Now()
		// Start with high transitions to get into flapping state
		for i := 0; i < 10; i++ {
			isUp := i%2 == 0
			m.RecordResult(isUp, 100, now.Add(time.Duration(-10+i)*time.Minute), 200, "", false)
		}
		isFlapping, _ := m.ComputeFlapping()
		if !isFlapping {
			t.Fatal("Expected flapping after alternating history")
		}

		// Now add enough stable results to drop below start threshold (25%) but above stop threshold (20%)
		// With window=10, we need transition% between 20 and 25 exclusive
		// Replace history: 8 up + 2 transitions = 2/9 = 22%
		m.mu.Lock()
		m.history = nil
		m.mu.Unlock()
		for i := 0; i < 8; i++ {
			m.RecordResult(true, 100, now.Add(time.Duration(-10+i)*time.Minute), 200, "", false)
		}
		m.RecordResult(false, 0, now.Add(-2*time.Minute), 0, "err", false)
		m.RecordResult(true, 100, now.Add(-1*time.Minute), 200, "", false)

		isFlapping, _ = m.ComputeFlapping()
		if !isFlapping {
			t.Error("Expected still flapping due to hysteresis (22% > stop threshold 20%)")
		}
	})

	t.Run("hysteresis_stops", func(t *testing.T) {
		cfg := MonitorConfig{
			ConfirmationThreshold: 3,
			CooldownMinutes:       30,
			FlapDetectionEnabled:  true,
			FlapWindowChecks:      10,
			FlapThresholdPercent:  25, // stop threshold = 20%
		}
		m := newTestMonitorWithConfig(cfg)
		now := time.Now()
		// Get into flapping state
		for i := 0; i < 10; i++ {
			isUp := i%2 == 0
			m.RecordResult(isUp, 100, now.Add(time.Duration(-20+i)*time.Minute), 200, "", false)
		}
		m.ComputeFlapping() // set flapping=true

		// Now make history very stable: 1 transition out of 9 = 11% < 20% stop threshold
		m.mu.Lock()
		m.history = nil
		m.mu.Unlock()
		for i := 0; i < 9; i++ {
			m.RecordResult(true, 100, now.Add(time.Duration(-10+i)*time.Minute), 200, "", false)
		}
		m.RecordResult(false, 0, now.Add(-1*time.Minute), 0, "err", false)

		isFlapping, changed := m.ComputeFlapping()
		if isFlapping {
			t.Error("Expected not flapping after dropping below stop threshold")
		}
		if !changed {
			t.Error("Expected changed=true when flapping stops")
		}
	})

	t.Run("disabled", func(t *testing.T) {
		cfg := MonitorConfig{
			ConfirmationThreshold: 3,
			CooldownMinutes:       30,
			FlapDetectionEnabled:  false,
			FlapWindowChecks:      21,
			FlapThresholdPercent:  25,
		}
		m := newTestMonitorWithConfig(cfg)
		now := time.Now()
		for i := 0; i < 10; i++ {
			isUp := i%2 == 0
			m.RecordResult(isUp, 100, now.Add(time.Duration(-10+i)*time.Minute), 200, "", false)
		}
		isFlapping, changed := m.ComputeFlapping()
		if isFlapping || changed {
			t.Error("Expected (false, false) when flap detection is disabled")
		}
	})

	t.Run("window_larger_than_history", func(t *testing.T) {
		cfg := MonitorConfig{
			ConfirmationThreshold: 3,
			CooldownMinutes:       30,
			FlapDetectionEnabled:  true,
			FlapWindowChecks:      21,
			FlapThresholdPercent:  25,
		}
		m := newTestMonitorWithConfig(cfg)
		now := time.Now()
		// Only 5 items, all alternating → 100% transitions
		for i := 0; i < 5; i++ {
			isUp := i%2 == 0
			m.RecordResult(isUp, 100, now.Add(time.Duration(-5+i)*time.Minute), 200, "", false)
		}
		isFlapping, _ := m.ComputeFlapping()
		if !isFlapping {
			t.Error("Expected flapping with high transitions (window clamped to history size)")
		}
	})

	t.Run("minimum_window_3", func(t *testing.T) {
		cfg := MonitorConfig{
			ConfirmationThreshold: 3,
			CooldownMinutes:       30,
			FlapDetectionEnabled:  true,
			FlapWindowChecks:      3,
			FlapThresholdPercent:  25,
		}
		m := newTestMonitorWithConfig(cfg)
		now := time.Now()
		m.RecordResult(true, 100, now.Add(-3*time.Minute), 200, "", false)
		m.RecordResult(false, 0, now.Add(-2*time.Minute), 0, "err", false)
		m.RecordResult(true, 100, now.Add(-1*time.Minute), 200, "", false)
		isFlapping, _ := m.ComputeFlapping()
		if !isFlapping {
			t.Error("Expected flapping with 100% transitions in 3-item window")
		}
	})

	t.Run("was_flapping_then_disabled", func(t *testing.T) {
		cfg := MonitorConfig{
			ConfirmationThreshold: 3,
			CooldownMinutes:       30,
			FlapDetectionEnabled:  true,
			FlapWindowChecks:      10,
			FlapThresholdPercent:  25,
		}
		m := newTestMonitorWithConfig(cfg)
		now := time.Now()
		for i := 0; i < 10; i++ {
			isUp := i%2 == 0
			m.RecordResult(isUp, 100, now.Add(time.Duration(-10+i)*time.Minute), 200, "", false)
		}
		m.ComputeFlapping() // set flapping=true
		if !m.IsFlapping() {
			t.Fatal("Expected flapping=true")
		}
		// Disable flap detection
		m.ApplyConfig(MonitorConfig{
			ConfirmationThreshold: 3,
			CooldownMinutes:       30,
			FlapDetectionEnabled:  false,
			FlapWindowChecks:      10,
			FlapThresholdPercent:  25,
		})
		isFlapping, changed := m.ComputeFlapping()
		if isFlapping {
			t.Error("Expected isFlapping=false after disabling")
		}
		if !changed {
			t.Error("Expected changed=true when flapping state cleared")
		}
	})
}

func TestMonitor_ApplyConfig(t *testing.T) {
	t.Run("updates_all_fields", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 1, CooldownMinutes: 0, FlapDetectionEnabled: false, FlapWindowChecks: 5, FlapThresholdPercent: 10})
		cfg := MonitorConfig{
			ConfirmationThreshold: 5,
			CooldownMinutes:       60,
			FlapDetectionEnabled:  true,
			FlapWindowChecks:      21,
			FlapThresholdPercent:  30,
		}
		m.ApplyConfig(cfg)
		m.mu.RLock()
		defer m.mu.RUnlock()
		if m.confirmationThreshold != 5 {
			t.Errorf("confirmationThreshold: got %d, want 5", m.confirmationThreshold)
		}
		if m.cooldownMinutes != 60 {
			t.Errorf("cooldownMinutes: got %d, want 60", m.cooldownMinutes)
		}
		if !m.flapDetectionEnabled {
			t.Error("flapDetectionEnabled: got false, want true")
		}
		if m.flapWindowChecks != 21 {
			t.Errorf("flapWindowChecks: got %d, want 21", m.flapWindowChecks)
		}
		if m.flapThresholdPercent != 30 {
			t.Errorf("flapThresholdPercent: got %d, want 30", m.flapThresholdPercent)
		}
	})

	t.Run("concurrent_safety", func(t *testing.T) {
		m := newTestMonitorWithConfig(MonitorConfig{ConfirmationThreshold: 3, CooldownMinutes: 30, FlapDetectionEnabled: true, FlapWindowChecks: 21, FlapThresholdPercent: 25})
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(2)
			go func() {
				defer wg.Done()
				m.ApplyConfig(MonitorConfig{ConfirmationThreshold: 5, CooldownMinutes: 10, FlapDetectionEnabled: false, FlapWindowChecks: 10, FlapThresholdPercent: 50})
			}()
			go func() {
				defer wg.Done()
				m.IncrementDown()
			}()
		}
		wg.Wait()
		// If we get here without race detector panic, test passes
	})
}
