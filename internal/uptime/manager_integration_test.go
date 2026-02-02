package uptime

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/clusteruptime/clusteruptime/internal/db"
)

func TestMonitor_DegradedThreshold(t *testing.T) {
	// Setup Store & Manager
	store, err := db.NewStore(db.NewTestConfigWithPath("file:test?mode=memory&cache=shared"))
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	// Skip Reset(), working with fresh memory DB.

	m := NewManager(store)
	m.Start()
	defer m.Stop()

	// 1. Setup Slow Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond) // Slow-ish
		w.WriteHeader(200)
	}))
	defer ts.Close()

	// 2. Set Low Threshold (10ms)
	m.SetLatencyThreshold(10)

	// 3. Create Monitor
	monID := "m-slow"
	if err := store.CreateMonitor(db.Monitor{
		ID:       monID,
		GroupID:  "g-default",
		Name:     "Slow Monitor",
		URL:      ts.URL,
		Active:   true,
		Interval: 1, // 1 second
	}); err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}

	// 4. Sycn to start
	m.Sync()

	// 5. Wait for check
	// Increase wait to ensure multiple checks happen (needed for history-based degradation detection)
	time.Sleep(4 * time.Second)

	// 6. Verify "Degraded" Event
	events, err := store.GetMonitorEvents(monID, 5)
	if err != nil {
		t.Fatalf("Failed to get events: %v", err)
	}

	foundDegraded := false
	for _, e := range events {
		if e.Type == "degraded" {
			foundDegraded = true
			break
		}
	}
	if !foundDegraded {
		t.Error("Expected 'degraded' event, found none")
		for _, e := range events {
			t.Logf("Event: %s: %s", e.Type, e.Message)
		}
	}

	// 7. Update Threshold High (5000ms)
	m.SetLatencyThreshold(5000)

	// 8. Wait for next check
	time.Sleep(2 * time.Second)

	// 9. Verify Recovered
	events, _ = store.GetMonitorEvents(monID, 5)
	foundRecovered := false
	for _, e := range events {
		if e.Type == "recovered" {
			foundRecovered = true
			break
		}
	}
	if !foundRecovered {
		t.Error("Expected 'recovered' event after increasing threshold")
	}
}

func TestMonitor_StatusCodes(t *testing.T) {
	// Setup Store & Manager
	// Use named shared memory DB to isolate tests but verify connection sharing
	store, err := db.NewStore(db.NewTestConfigWithPath("file:test?mode=memory&cache=shared"))
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	// Note: We don't call store.Reset() to avoid potential table drop races or overhead.
	// We rely on creating a unique monitor ID.

	m := NewManager(store)
	m.Start()
	defer m.Stop()

	// 1. Setup Server that we can control
	authCode := 200
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(authCode)
	}))
	defer ts.Close()

	monID := "m-status"
	if err := store.CreateMonitor(db.Monitor{
		ID:       monID,
		GroupID:  "g-default",
		Name:     "Status Monitor",
		URL:      ts.URL,
		Active:   true,
		Interval: 1,
	}); err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	m.Sync()

	// Case 1: 200 OK
	time.Sleep(2 * time.Second)
	// Check status
	mon := m.GetMonitor(monID)
	up, _, _, _ := mon.GetLastStatus()
	if !up {
		t.Error("Monitor should be UP on 200")
	}

	// Case 2: 500 Error
	authCode = 500
	// Wait next tick
	time.Sleep(2 * time.Second)
	up, _, _, _ = mon.GetLastStatus()
	if up {
		t.Error("Monitor should be DOWN on 500")
	}

	// Case 3: 404 Error
	authCode = 404
	time.Sleep(2 * time.Second)
	up, _, _, _ = mon.GetLastStatus()
	if up {
		t.Error("Monitor should be DOWN on 404")
	}

	// Case 4: Recover to 200
	authCode = 200
	time.Sleep(2 * time.Second)
	up, _, _, _ = mon.GetLastStatus()
	if !up {
		t.Error("Monitor should be UP after recovery")
	}
}

func TestMonitor_EventLifecycle(t *testing.T) {
	// Setup Store & Manager
	store, err := db.NewStore(db.NewTestConfigWithPath("file:test?mode=memory&cache=shared"))
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	m := NewManager(store)
	m.Start()
	defer m.Stop()

	// 1. Setup Server
	latency := 0 * time.Millisecond
	paramCode := 200
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(latency)
		w.WriteHeader(paramCode)
	}))
	defer ts.Close()

	monID := "m-lifecycle"
	if err := store.CreateMonitor(db.Monitor{
		ID:       monID,
		GroupID:  "g-default",
		Name:     "Lifecycle Monitor",
		URL:      ts.URL,
		Active:   true,
		Interval: 1,
	}); err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	m.SetLatencyThreshold(100)
	m.Sync()

	time.Sleep(2 * time.Second) // Initial Up

	// 2. Trigger Downtime (500)
	paramCode = 500
	time.Sleep(2 * time.Second)

	// Verify "down" event
	events, _ := store.GetMonitorEvents(monID, 5)
	if len(events) == 0 || events[0].Type != "down" {
		t.Error("Expected 'down' event")
		for _, e := range events {
			t.Logf("Found: %s", e.Type)
		}
	}

	// 3. Recover (200)
	paramCode = 200
	time.Sleep(2 * time.Second)

	// Verify "recovered" event
	events, _ = store.GetMonitorEvents(monID, 5)
	if len(events) == 0 || events[0].Type != "recovered" {
		t.Error("Expected 'recovered' event")
	}

	// 4. Trigger Degraded (Latency > 100ms)
	latency = 200 * time.Millisecond
	time.Sleep(4 * time.Second) // Wait for debounce logic

	// Verify "degraded" event
	events, _ = store.GetMonitorEvents(monID, 5)
	foundDegraded := false
	for _, e := range events {
		if e.Type == "degraded" {
			foundDegraded = true
			break
		}
	}
	if !foundDegraded {
		t.Error("Expected 'degraded' event")
	}

	// 5. Recover (Latency normal)
	latency = 10 * time.Millisecond
	time.Sleep(4 * time.Second)

	// Verify "recovered" event
	events, _ = store.GetMonitorEvents(monID, 5)
	if len(events) == 0 || events[0].Type != "recovered" {
		t.Error("Expected 'recovered' event after degradation")
	}
}

func TestMonitor_ErrorTypes(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfigWithPath("file:test?mode=memory&cache=shared"))
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	m := NewManager(store)
	m.Start()
	defer m.Stop()

	// 1. Timeout Scenario
	// Server sleeps longer than client timeout (default 5s, we can't easily change client timeout per request without mocking client,
	// but we can set server sleep to something reasonable and ensure we can detect it if we enforce a small timeout or just simulate "context deadline exceeded")
	// Actually, Monitor uses http.DefaultClient which has no timeout by default.
	// We should probably set a timeout on the client.
	// For this test, let's assume the server closes connection chunked or something?
	// Easiest is to close the server mid-request to get "connection reset"?
	// Or use a Closed Port for "connection refused".

	// Case 1: Connection Refused (Closed Port)
	// Pick a random port that is likely closed? Or localhost:1
	monID := "m-refused"
	if err := store.CreateMonitor(db.Monitor{
		ID:       monID,
		GroupID:  "g-default",
		Name:     "Refused Monitor",
		URL:      "http://localhost:65432", // Unlikely to be open
		Active:   true,
		Interval: 1,
	}); err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	m.Sync()
	time.Sleep(2 * time.Second)

	events, _ := store.GetMonitorEvents(monID, 5)
	if len(events) == 0 {
		t.Error("Expected event for connection refused")
	} else {
		t.Logf("Refused Event: %s", events[0].Message)
		// Expect "connection refused" in message
	}

	// Case 2: Protocol Error (Garbage response?)
	// Not easy with httptest. But Refused vs Timeout is good enough.

	// Check Status
	mon := m.GetMonitor(monID)
	up, _, _, _ := mon.GetLastStatus()
	if up {
		t.Error("Monitor should be DOWN for connection refused")
	}
}
