package notifications

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
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

func sampleEvent() NotificationEvent {
	return NotificationEvent{
		MonitorID:   "mon-123",
		MonitorName: "Test Monitor",
		MonitorURL:  "https://example.com",
		Type:        EventDown,
		Message:     "Connection refused",
		Time:        time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC),
	}
}

func TestWebhookNotifier_Payload(t *testing.T) {
	var received map[string]interface{}
	var contentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	config := `{"webhookUrl":"` + srv.URL + `"}`
	notifier := NewWebhookNotifier(config)

	if err := notifier.Send(sampleEvent()); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	// Verify all expected fields
	expectedFields := []string{"event", "monitorId", "monitorName", "monitorUrl", "message", "timestamp"}
	for _, field := range expectedFields {
		if _, ok := received[field]; !ok {
			t.Errorf("missing field %q in webhook payload", field)
		}
	}

	if received["event"] != "down" {
		t.Errorf("expected event 'down', got %v", received["event"])
	}
	if received["monitorId"] != "mon-123" {
		t.Errorf("expected monitorId 'mon-123', got %v", received["monitorId"])
	}
	if received["monitorName"] != "Test Monitor" {
		t.Errorf("expected monitorName 'Test Monitor', got %v", received["monitorName"])
	}
	if received["monitorUrl"] != "https://example.com" {
		t.Errorf("expected monitorUrl 'https://example.com', got %v", received["monitorUrl"])
	}
	if received["message"] != "Connection refused" {
		t.Errorf("expected message 'Connection refused', got %v", received["message"])
	}

	// Verify timestamp is RFC3339
	ts, ok := received["timestamp"].(string)
	if !ok {
		t.Fatal("timestamp is not a string")
	}
	if _, err := time.Parse(time.RFC3339, ts); err != nil {
		t.Errorf("timestamp is not valid RFC3339: %s", ts)
	}
}

func TestWebhookNotifier_HTTPMethod(t *testing.T) {
	var method string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	config := `{"webhookUrl":"` + srv.URL + `"}`
	notifier := NewWebhookNotifier(config)

	if err := notifier.Send(sampleEvent()); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if method != "POST" {
		t.Errorf("expected POST, got %s", method)
	}
}

func TestWebhookNotifier_MissingURL(t *testing.T) {
	notifier := NewWebhookNotifier(`{}`)
	if err := notifier.Send(sampleEvent()); err == nil {
		t.Error("expected error for missing webhookUrl")
	}
}

func TestWebhookNotifier_EmptyURL(t *testing.T) {
	notifier := NewWebhookNotifier(`{"webhookUrl":""}`)
	if err := notifier.Send(sampleEvent()); err == nil {
		t.Error("expected error for empty webhookUrl")
	}
}

func TestWebhookNotifier_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	config := `{"webhookUrl":"` + srv.URL + `"}`
	notifier := NewWebhookNotifier(config)

	if err := notifier.Send(sampleEvent()); err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestWebhookNotifier_InvalidScheme(t *testing.T) {
	notifier := NewWebhookNotifier(`{"webhookUrl":"ftp://example.com"}`)
	if err := notifier.Send(sampleEvent()); err == nil {
		t.Error("expected error for non-HTTP scheme")
	}
}

func TestWebhookNotifier_AllEventTypes(t *testing.T) {
	cases := []struct {
		eventType EventType
		expected  string
	}{
		{EventDown, "down"},
		{EventUp, "up"},
		{EventDegraded, "degraded"},
		{EventSSLExpiring, "ssl_expiring"},
		{EventFlapping, "flapping"},
		{EventStabilized, "stabilized"},
	}

	for _, tc := range cases {
		t.Run(string(tc.eventType), func(t *testing.T) {
			var received map[string]interface{}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(body, &received)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			config := `{"webhookUrl":"` + srv.URL + `"}`
			notifier := NewWebhookNotifier(config)

			event := sampleEvent()
			event.Type = tc.eventType

			if err := notifier.Send(event); err != nil {
				t.Fatalf("Send failed: %v", err)
			}

			if received["event"] != tc.expected {
				t.Errorf("expected event %q, got %v", tc.expected, received["event"])
			}
		})
	}
}

func TestWebhookNotifier_NoExtraFields(t *testing.T) {
	var received map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	config := `{"webhookUrl":"` + srv.URL + `"}`
	notifier := NewWebhookNotifier(config)

	if err := notifier.Send(sampleEvent()); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	allowedFields := map[string]bool{
		"event": true, "monitorId": true, "monitorName": true,
		"monitorUrl": true, "message": true, "timestamp": true,
	}
	for key := range received {
		if !allowedFields[key] {
			t.Errorf("unexpected field %q in webhook payload", key)
		}
	}
	if len(received) != 6 {
		t.Errorf("expected exactly 6 fields, got %d", len(received))
	}
}

func TestSendDirect_Webhook(t *testing.T) {
	var received map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	config := `{"webhookUrl":"` + srv.URL + `"}`
	if err := SendDirect("webhook", config, sampleEvent()); err != nil {
		t.Fatalf("SendDirect failed: %v", err)
	}

	if received["event"] != "down" {
		t.Errorf("expected event 'down', got %v", received["event"])
	}
}

func TestSendDirect_Slack(t *testing.T) {
	var received map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	config := `{"webhookUrl":"` + srv.URL + `"}`
	if err := SendDirect("slack", config, sampleEvent()); err != nil {
		t.Fatalf("SendDirect failed: %v", err)
	}

	// Slack format has "text" and "attachments"
	if _, ok := received["text"]; !ok {
		t.Error("expected 'text' field in Slack payload")
	}
	if _, ok := received["attachments"]; !ok {
		t.Error("expected 'attachments' field in Slack payload")
	}
}

func TestSendDirect_UnsupportedType(t *testing.T) {
	if err := SendDirect("carrier_pigeon", `{}`, sampleEvent()); err == nil {
		t.Error("expected error for unsupported type")
	}
}
