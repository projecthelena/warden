package notifications

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

func TestBatchPreservesTransitionsAndBoundsSummary(t *testing.T) {
	events := make([]NotificationEvent, 100)
	for i := range events {
		events[i] = NotificationEvent{MonitorID: "one", MonitorName: strings.Repeat("界", 300), Type: EventDown, Message: strings.Repeat("界", 3000)}
	}
	events[1].Type = EventUp
	batch := batchEvents(events)
	if len(batch.Events) != 100 || batch.Events[1].Type != EventUp {
		t.Fatal("lost original transitions")
	}
	if len([]rune(batch.Message)) > 2400 || !strings.Contains(batch.Message, "92 more") {
		t.Fatal("unbounded summary")
	}
	if got := batchEvents(events[:1]); got.Type != EventDown || got.MonitorID != "one" {
		t.Fatal("single event changed")
	}
}

func TestWorkerGroupsBurstAndFlushesOnClose(t *testing.T) {
	store := newTestStore(t)
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close test store: %v", err)
		}
	})
	received := make(chan map[string]interface{}, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		received <- payload
	}))
	defer server.Close()
	config, _ := json.Marshal(map[string]string{"webhookUrl": server.URL})
	if err := store.CreateNotificationChannel(db.NotificationChannel{ID: "batch", Name: "Batch", Type: "webhook", Config: string(config), Enabled: true}); err != nil {
		t.Fatal(err)
	}
	svc := NewService(store)
	done := make(chan struct{})
	go func() { svc.runWorker(20 * time.Millisecond); close(done) }()
	svc.Enqueue(NotificationEvent{MonitorID: "a", Type: EventDown})
	svc.Enqueue(NotificationEvent{MonitorID: "b", Type: EventUp})
	select {
	case payload := <-received:
		if payload["event"] != "batch" || len(payload["events"].([]interface{})) != 2 {
			t.Fatalf("unexpected payload: %v", payload)
		}
		first := payload["events"].([]interface{})[0].(map[string]interface{})
		if first["event"] != "down" || first["monitorId"] != "a" {
			t.Fatalf("batch child lost webhook schema: %v", first)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("batch never delivered")
	}
	svc.Enqueue(NotificationEvent{MonitorID: "c", Type: EventDown})
	close(svc.queue)
	<-done
	select {
	case payload := <-received:
		if payload["monitorId"] != "c" {
			t.Fatalf("unexpected final delivery: %v", payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pending event lost on close")
	}
	select {
	case <-received:
		t.Fatal("duplicate delivery")
	default:
	}
}

func TestBatchKeepsCriticalUpdatesVisible(t *testing.T) {
	events := make([]NotificationEvent, 20)
	for i := range events {
		events[i] = NotificationEvent{MonitorName: "Recovered service", Type: EventUp}
	}
	events[19] = NotificationEvent{MonitorName: "Critical API", Type: EventDown, Message: "Unavailable"}
	batch := batchEvents(events)
	if !strings.HasPrefix(batch.Message, "Critical API") {
		t.Fatal("critical failure hidden behind recoveries")
	}
	if notificationSeverity(batch) != EventDown {
		t.Fatal("mixed batch lost critical severity")
	}
	if batch.Events[19].MonitorName != "Critical API" {
		t.Fatal("machine event order changed")
	}
}
