package uptime

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

func newEventMonitor(t *testing.T) *Monitor {
	t.Helper()
	return NewMonitor("m1", db.MonitorTypeHTTP, "g1", "Test", "https://example.com", time.Minute, make(chan Job, 1), time.Now(), nil)
}

// A monitor that stays down produces a failed check every interval. Writing each one
// leaves hundreds of rows saying the same thing, each carrying its own copy of the
// response body.
func TestShouldRecordEventSkipsAnUnchangedRepeat(t *testing.T) {
	m := newEventMonitor(t)

	if !m.ShouldRecordEvent("down", "Monitor is down", "connection refused") {
		t.Fatal("the first failure has to be recorded")
	}
	for i := 0; i < 100; i++ {
		if m.ShouldRecordEvent("down", "Monitor is down", "connection refused") {
			t.Fatalf("repeat %d said the same thing and should not be recorded again", i)
		}
	}
}

// The drill-down exists to show what changed during an outage, so a different reason has
// to survive the deduplication.
func TestShouldRecordEventNoticesAChangedReason(t *testing.T) {
	m := newEventMonitor(t)
	m.ShouldRecordEvent("down", "Monitor is down", "connection refused")

	if !m.ShouldRecordEvent("down", "Monitor is down", "i/o timeout") {
		t.Error("a different error is worth recording")
	}
	if !m.ShouldRecordEvent("down", "Monitor is down (Status: 503)", "i/o timeout") {
		t.Error("a different message is worth recording")
	}
	if !m.ShouldRecordEvent("degraded", "Monitor is down (Status: 503)", "i/o timeout") {
		t.Error("a different type is worth recording")
	}
}

// The trap this design has to avoid: if a recovery did not reset the key, a second
// outage that reads exactly like the first would leave no event at all.
func TestShouldRecordEventRecordsASecondOutage(t *testing.T) {
	m := newEventMonitor(t)

	if !m.ShouldRecordEvent("down", "Monitor is down", "connection refused") {
		t.Fatal("the first outage has to be recorded")
	}
	if !m.ShouldRecordEvent("recovered", "Monitor recovered", "") {
		t.Fatal("the recovery has to be recorded")
	}
	if !m.ShouldRecordEvent("down", "Monitor is down", "connection refused") {
		t.Fatal("a second outage has to be recorded even though it reads like the first")
	}
}

// Nothing in the key survives a restart, so the first event after one is written again.
// One extra row per restart is the price of not reading the database on every check.
func TestShouldRecordEventStartsFreshOnANewMonitor(t *testing.T) {
	first := newEventMonitor(t)
	first.ShouldRecordEvent("down", "Monitor is down", "connection refused")

	restarted := newEventMonitor(t)
	if !restarted.ShouldRecordEvent("down", "Monitor is down", "connection refused") {
		t.Error("a monitor that has just been scheduled has nothing to compare against")
	}
}

// The fields are joined into one key, so values must not be able to run together and
// look identical when they are not.
func TestShouldRecordEventDoesNotConfuseFieldBoundaries(t *testing.T) {
	m := newEventMonitor(t)

	m.ShouldRecordEvent("down", "ab", "c")
	if !m.ShouldRecordEvent("down", "a", "bc") {
		t.Error("two different splits of the same characters are different events")
	}
}

// resultProcessor writes events while the scheduler reads the monitor, so the key lives
// under the same lock as the rest of the state.
func TestShouldRecordEventIsSafeUnderConcurrency(t *testing.T) {
	m := newEventMonitor(t)

	recorded := make(chan bool, 50)
	for i := 0; i < 50; i++ {
		go func() { recorded <- m.ShouldRecordEvent("down", "Monitor is down", "connection refused") }()
	}

	count := 0
	for i := 0; i < 50; i++ {
		if <-recorded {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one of the identical events to be recorded, got %d", count)
	}
}

// The point of all this, measured end to end: a monitor that stays down through many
// checks leaves one event, not one per check.
func TestSustainedOutageWritesOneEvent(t *testing.T) {
	n := testDBCounter.Add(1)
	store, err := db.NewStore(db.NewTestConfigWithPath(fmt.Sprintf("file:sustained_%d?mode=memory&cache=shared", n)))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	setIntegrationTestDefaults(store)

	// A port nothing listens on, so every check fails the same way.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to open listener: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	m := NewManager(store)
	m.Start()
	defer m.Stop()

	if err := store.CreateMonitor(db.Monitor{
		ID: "m-sustained", Type: db.MonitorTypeTCP, GroupID: "g-default",
		Name: "Sustained", URL: addr, Active: true, Interval: 1,
	}); err != nil {
		t.Fatalf("failed to create monitor: %v", err)
	}
	m.Sync()

	// Let it fail several times over.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if checks, err := store.GetMonitorChecks("m-sustained", 50); err == nil && len(checks) >= 4 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	checks, err := store.GetMonitorChecks("m-sustained", 50)
	if err != nil {
		t.Fatalf("failed to read checks: %v", err)
	}
	if len(checks) < 4 {
		t.Fatalf("expected several failed checks to have run, got %d", len(checks))
	}

	events, err := store.GetMonitorEvents("m-sustained", 50)
	if err != nil {
		t.Fatalf("failed to read events: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("expected %d identical failures to leave 1 event, got %d", len(checks), len(events))
	}
}
