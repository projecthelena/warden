package uptime

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/notifications"
)

// spyNotifier records what would have gone out on the wire.
type spyNotifier struct {
	mu     sync.Mutex
	events []notifications.NotificationEvent
}

func (s *spyNotifier) Start() {}

func (s *spyNotifier) Enqueue(e notifications.NotificationEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
}

func (s *spyNotifier) SendDigest([]db.DigestEvent) {}

func (s *spyNotifier) byType(t notifications.EventType) []notifications.NotificationEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []notifications.NotificationEvent
	for _, e := range s.events {
		if e.Type == t {
			out = append(out, e)
		}
	}
	return out
}

func waitForOpenOutage(t *testing.T, store *db.Store, d time.Duration) db.OpenOutage {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		open, err := store.GetOpenOutages()
		if err == nil && len(open) > 0 {
			return open[0]
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("no outage opened within the timeout")
	return db.OpenOutage{}
}

// This is the boundary test for the whole phase: a monitor going down must open an outage
// and say nothing. Restoring the old "notify on the transition" behaviour makes this fail,
// which a test that only watches the evaluator would not.
func TestDownDoesNotNotifyOnTheTransition(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfigWithPath("file:alerts_integration_1?mode=memory&cache=shared"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer func() { _ = store.Close() }()

	_ = store.SetSetting("notification.confirmation_threshold", "1")
	_ = store.SetSetting("notification.flap_detection_enabled", "false")
	_ = store.SetSetting("notification.cooldown_minutes", "0")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	spy := &spyNotifier{}
	m := NewManager(store)
	m.notifier = spy
	m.Start()
	defer m.Stop()

	if err := store.CreateMonitor(db.Monitor{
		ID: "m-down", GroupID: "g-default", Name: "Always 503",
		URL: ts.URL, Active: true, Interval: 1,
	}); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}
	m.Sync()

	outage := waitForOpenOutage(t, store, 8*time.Second)

	// The outage exists and nothing was said about it.
	if sent := spy.byType(notifications.EventDown); len(sent) != 0 {
		t.Fatalf("a down alert went out on the transition: %+v", sent)
	}
	if outage.NotifiedAt != nil {
		t.Fatalf("outage was stamped as announced without the evaluator running")
	}

	// Once the outage is old enough, the evaluator speaks — exactly once.
	m.evaluateAlerts(outage.StartTime.Add(3 * time.Minute))
	sent := spy.byType(notifications.EventDown)
	if len(sent) != 1 {
		t.Fatalf("expected exactly 1 down alert after the sustained window, got %d", len(sent))
	}
	if sent[0].MonitorName != "Always 503" {
		t.Errorf("alert names the wrong monitor: %q", sent[0].MonitorName)
	}

	m.evaluateAlerts(outage.StartTime.Add(4 * time.Minute))
	if sent := spy.byType(notifications.EventDown); len(sent) != 1 {
		t.Errorf("the same outage was announced %d times", len(sent))
	}
}

// The mirror of the rule: a blip that resolves inside the silent window produces no "down"
// and, just as importantly, no "recovered" either.
func TestShortOutageProducesNoMessagesAtAll(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfigWithPath("file:alerts_integration_2?mode=memory&cache=shared"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer func() { _ = store.Close() }()

	_ = store.SetSetting("notification.confirmation_threshold", "1")
	_ = store.SetSetting("notification.flap_detection_enabled", "false")
	_ = store.SetSetting("notification.cooldown_minutes", "0")

	var failing bool
	var mu sync.Mutex
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		down := failing
		mu.Unlock()
		if down {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	spy := &spyNotifier{}
	m := NewManager(store)
	m.notifier = spy
	m.Start()
	defer m.Stop()

	if err := store.CreateMonitor(db.Monitor{
		ID: "m-blip", GroupID: "g-default", Name: "Blip",
		URL: ts.URL, Active: true, Interval: 1,
	}); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}
	m.Sync()

	// Let it settle as up, then fail briefly, then recover — all well inside three minutes.
	time.Sleep(2 * time.Second)
	mu.Lock()
	failing = true
	mu.Unlock()

	waitForOpenOutage(t, store, 8*time.Second)

	mu.Lock()
	failing = false
	mu.Unlock()

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		open, _ := store.GetOpenOutages()
		if len(open) == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if open, _ := store.GetOpenOutages(); len(open) != 0 {
		t.Fatal("the outage never closed")
	}
	if sent := spy.byType(notifications.EventDown); len(sent) != 0 {
		t.Errorf("a blip produced %d down alerts: %+v", len(sent), sent)
	}
	if sent := spy.byType(notifications.EventUp); len(sent) != 0 {
		t.Errorf("a blip nobody was told about produced %d recovery messages: %+v", len(sent), sent)
	}
}
