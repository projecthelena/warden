package uptime

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/notifications"
)

func TestCorrelatedRecoveryWaitsForWholeGroup(t *testing.T) {
	m, store := newAlertTestManager(t)
	spy := &spyNotifier{}
	m.notifier = spy
	if err := store.CreateMonitor(db.Monitor{ID: "m2", GroupID: "g1", Name: "Second", URL: "https://second.example", Interval: 60, Active: true}); err != nil {
		t.Fatal(err)
	}
	m.monitors["m2"] = NewMonitor("m2", db.MonitorTypeHTTP, "g1", "Second", "https://second.example", time.Minute, m.jobQueue, time.Now(), nil)
	now := time.Now().UTC()
	for _, id := range []string{"m1", "m2"} {
		if err := store.CreateOutage(id, "down", "unavailable"); err != nil {
			t.Fatal(err)
		}
	}
	outages, err := store.GetOpenOutages()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.MarkOutagesNotified(outageIDs(outages), now, "group-test"); err != nil {
		t.Fatal(err)
	}
	recoverMonitor := func(id string, at time.Time) {
		event := notifications.NotificationEvent{MonitorID: id, MonitorName: id, Type: notifications.EventUp, Time: at}
		data, _ := json.Marshal(event)
		if err := store.CloseOutageQueueRecovery(id, string(data), at, true); err != nil {
			t.Fatal(err)
		}
	}
	recordHealthy := func(id string, start time.Time) {
		for i := 0; i <= 3; i++ {
			m.GetMonitor(id).RecordResult(true, 10, start.Add(time.Duration(i)*time.Minute), 200, "", false)
		}
	}
	recoverMonitor("m1", now)
	recordHealthy("m1", now)
	m.evaluateRecoveries(now.Add(3 * time.Minute))
	if len(spy.byType(notifications.EventUp)) != 0 {
		t.Fatal("partial recovery interrupted before remaining member recovered")
	}
	recoverMonitor("m2", now.Add(time.Minute))
	m.evaluateRecoveries(now.Add(3 * time.Minute))
	if len(spy.byType(notifications.EventUp)) != 0 {
		t.Fatal("group sent before second monitor was stable")
	}
	recordHealthy("m2", now.Add(time.Minute))
	m.GetMonitor("m1").RecordResult(true, 10, now.Add(4*time.Minute), 200, "", false)
	m.evaluateRecoveries(now.Add(4 * time.Minute))
	if got := len(spy.byType(notifications.EventUp)); got != 2 {
		t.Fatalf("expected both recoveries in same delivery window, got %d", got)
	}
	m.evaluateRecoveries(now.Add(4 * time.Minute))
	if got := len(spy.byType(notifications.EventUp)); got != 2 {
		t.Fatal("group recovery repeated")
	}
}
