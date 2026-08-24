package uptime

import (
	"fmt"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

// A monitor's group decides which maintenance window silences it and which outage cluster
// it is counted in, and the manager keeps its own copy of it in memory. Sync used to set
// that copy once, at creation, so a monitor moved to another group went on being silenced
// by the window on the group it had left until the process restarted.
func TestSyncPicksUpAGroupMove(t *testing.T) {
	n := testDBCounter.Add(1)
	store, err := db.NewStore(db.NewTestConfigWithPath(fmt.Sprintf("file:group_move_%d?mode=memory&cache=shared", n)))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	m := NewManager(store)
	t.Cleanup(m.Reset)

	if err := store.CreateGroup(db.Group{ID: "g-prod", Name: "Production"}); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if err := store.CreateMonitor(db.Monitor{
		ID: "m-move", GroupID: "g-default", Name: "Movable", URL: "http://example.com",
		Active: true, Interval: 60,
	}); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}
	if err := store.BatchInsertChecks([]db.CheckResult{
		{MonitorID: "m-move", Status: "up", Latency: 20, Timestamp: time.Now()},
	}); err != nil {
		t.Fatalf("BatchInsertChecks: %v", err)
	}

	m.Sync()
	before := m.GetMonitor("m-move")
	if before == nil {
		t.Fatal("monitor should be running after the first sync")
	}
	if before.GetGroupID() != "g-default" {
		t.Fatalf("expected g-default, got %s", before.GetGroupID())
	}
	if len(before.GetHistory()) == 0 {
		t.Fatal("expected the monitor to hydrate its history")
	}

	if err := store.MoveMonitorToGroup("m-move", "g-prod"); err != nil {
		t.Fatalf("MoveMonitorToGroup: %v", err)
	}
	m.Sync()

	after := m.GetMonitor("m-move")
	if after == nil {
		t.Fatal("monitor should still be running after the move")
	}
	if after.GetGroupID() != "g-prod" {
		t.Errorf("expected the running monitor to be in g-prod, got %s", after.GetGroupID())
	}
	// Regrouping does not change how the monitor is checked, so it must be applied in
	// place. A restart here would throw away the confirmation state and the history that
	// decide whether the next failed check is worth alerting on.
	if after != before {
		t.Error("the monitor was restarted for a change that does not affect checking")
	}
	if len(after.GetHistory()) == 0 {
		t.Error("the move cost the monitor its history")
	}
}
