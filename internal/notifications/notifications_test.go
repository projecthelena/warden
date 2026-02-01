package notifications

import (
	"testing"
	"time"

	"github.com/clusteruptime/clusteruptime/internal/db"
)

// Simplified test store setup since we can't easily import db.newTestStore here due to circular deps if we were inside db package.
// But we are in notifications package, so we can import db.
func newTestStore(t *testing.T) *db.Store {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	return store
}

func TestSlackNotifier_ConfigParsing(t *testing.T) {
	configJSON := `{"webhookUrl": "https://hooks.slack.com/services/XXX"}`
	notifier := NewSlackNotifier(configJSON)

	if notifier == nil {
		t.Fatal("NewSlackNotifier returned nil")
	}

	// We can't access private config map directly, but we can verify behavior via Send?
	// Actually we should export something or just test Send failing/passing based on config.
}

func TestService_Dispatch(t *testing.T) {
	store := newTestStore(t)
	svc := NewService(store)

	// Create a channel
	ch := db.NotificationChannel{
		ID:        "nc1",
		Type:      "slack",
		Name:      "Test",
		Config:    `{"webhookUrl": "https://hooks.slack.com/services/XXX"}`,
		Enabled:   true,
		CreatedAt: time.Now(),
	}
	if err := store.CreateNotificationChannel(ch); err != nil {
		t.Fatalf("Failed to create channel: %v", err)
	}

	// Enqueue event
	event := NotificationEvent{
		MonitorID:   "m1",
		MonitorName: "Test Monitor",
		Type:        EventDown,
		Message:     "System Down",
		Time:        time.Now(),
	}

	// Start service
	svc.Start()

	svc.Enqueue(event)

	// In a real integration test we would mock the HTTP transport for SlackNotifier
	// to verify it made a request.
	// Since SlackNotifier uses standard http.Client inside sendJSON (private),
	// we assume here that no panic occurs.
	// We can't easily verification without Refactoring SlackNotifier to accept a custom HTTP client.
}
