package notifications

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

// --- Digest tests ---

func TestEventSeverity(t *testing.T) {
	// Verify severity ordering: down < degraded < flapping < ssl_expiring < stabilized < up < unknown
	ordered := []string{"down", "degraded", "flapping", "ssl_expiring", "stabilized", "up"}
	for i := 0; i < len(ordered)-1; i++ {
		if eventSeverity(ordered[i]) >= eventSeverity(ordered[i+1]) {
			t.Errorf("%s (severity %d) should be more severe than %s (severity %d)",
				ordered[i], eventSeverity(ordered[i]), ordered[i+1], eventSeverity(ordered[i+1]))
		}
	}
	// Unknown should be least severe
	if eventSeverity("up") >= eventSeverity("unknown") {
		t.Error("up should be more severe than unknown")
	}
}

func TestEventEmoji(t *testing.T) {
	types := []string{"down", "degraded", "ssl_expiring", "flapping", "stabilized", "up"}
	for _, typ := range types {
		emoji := eventEmoji(typ)
		if emoji == "" || emoji == ":grey_question:" {
			t.Errorf("expected non-default emoji for %q, got %q", typ, emoji)
		}
	}
	if eventEmoji("unknown") != ":grey_question:" {
		t.Errorf("expected :grey_question: for unknown type")
	}
}

func TestEventLabel(t *testing.T) {
	cases := map[string]string{
		"down":         "Down",
		"degraded":     "Degraded",
		"ssl_expiring": "SSL Expiring",
		"flapping":     "Flapping",
		"stabilized":   "Stabilized",
		"up":           "Up",
	}
	for typ, expected := range cases {
		if got := eventLabel(typ); got != expected {
			t.Errorf("eventLabel(%q) = %q, want %q", typ, got, expected)
		}
	}
	if got := eventLabel("custom"); got != "custom" {
		t.Errorf("eventLabel('custom') = %q, want 'custom'", got)
	}
}

func TestSlackDigest_BlockKitFormat(t *testing.T) {
	var received map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	config := `{"webhookUrl":"` + srv.URL + `"}`
	notifier := NewSlackNotifier(config)

	summary := digestSummary{
		TotalEvents:  5,
		MonitorCount: 2,
		Monitors: []digestMonitor{
			{
				Name:     "API Server",
				URL:      "https://api.example.com",
				Events:   []digestEventCount{{Type: "down", Count: 3}, {Type: "up", Count: 2}},
				Severity: 0,
			},
			{
				Name:     "Web App",
				URL:      "https://app.example.com",
				Events:   []digestEventCount{{Type: "degraded", Count: 1}},
				Severity: 1,
			},
		},
		Date: time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC),
	}

	if err := notifier.sendDigest(summary); err != nil {
		t.Fatalf("sendDigest failed: %v", err)
	}

	// Verify fallback text exists
	if _, ok := received["text"]; !ok {
		t.Error("expected 'text' fallback field")
	}

	// Verify blocks exist
	blocks, ok := received["blocks"].([]interface{})
	if !ok {
		t.Fatal("expected 'blocks' array")
	}

	// header + context + divider + 2 monitor sections = 5 blocks
	if len(blocks) != 5 {
		t.Fatalf("expected 5 blocks, got %d", len(blocks))
	}

	// Verify block types
	expectedTypes := []string{"header", "context", "divider", "section", "section"}
	for i, expectedType := range expectedTypes {
		block := blocks[i].(map[string]interface{})
		if block["type"] != expectedType {
			t.Errorf("block[%d]: expected type %q, got %v", i, expectedType, block["type"])
		}
	}

	// Verify context contains date and stats
	ctx := blocks[1].(map[string]interface{})
	elements := ctx["elements"].([]interface{})
	ctxText := elements[0].(map[string]interface{})["text"].(string)
	if !strings.Contains(ctxText, "March 15, 2026") {
		t.Errorf("context should contain date, got: %s", ctxText)
	}
	if !strings.Contains(ctxText, "5 events across 2 monitors") {
		t.Errorf("context should contain stats, got: %s", ctxText)
	}

	// Verify first monitor section contains name and event emojis
	section := blocks[3].(map[string]interface{})
	sectionText := section["text"].(map[string]interface{})["text"].(string)
	if !strings.Contains(sectionText, "*API Server*") {
		t.Errorf("expected monitor name in section, got: %s", sectionText)
	}
	if !strings.Contains(sectionText, "Down `3x`") {
		t.Errorf("expected 'Down `3x`' in section, got: %s", sectionText)
	}
}

// TestSlackDigest_AppURLLinks verifies that when AppURL is set the digest header carries a
// "View full report" link and each monitor name becomes a clickable link to its day page.
// This is the wire-up that lets the daily Slack digest function as a real drill-down entry point.
func TestSlackDigest_AppURLLinks(t *testing.T) {
	var received map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	notifier := NewSlackNotifier(`{"webhookUrl":"` + srv.URL + `"}`)

	summary := digestSummary{
		TotalEvents:  2,
		MonitorCount: 1,
		Monitors: []digestMonitor{
			{
				ID:       "mon-abc",
				Name:     "API | Server", // intentionally includes a pipe to exercise escaping
				URL:      "https://api.example.com",
				Events:   []digestEventCount{{Type: "down", Count: 2}},
				Severity: 0,
			},
		},
		Date:   time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC),
		AppURL: "https://warden.example.com",
	}

	if err := notifier.sendDigest(summary); err != nil {
		t.Fatalf("sendDigest failed: %v", err)
	}

	blocks := received["blocks"].([]interface{})
	ctxText := blocks[1].(map[string]interface{})["elements"].([]interface{})[0].(map[string]interface{})["text"].(string)
	if !strings.Contains(ctxText, "https://warden.example.com/incidents?date=2026-03-15") {
		t.Errorf("context should embed incidents link, got: %s", ctxText)
	}
	if !strings.Contains(ctxText, "View full report") {
		t.Errorf("context should contain link label, got: %s", ctxText)
	}

	sectionText := blocks[3].(map[string]interface{})["text"].(map[string]interface{})["text"].(string)
	if !strings.Contains(sectionText, "https://warden.example.com/monitors/mon-abc?date=2026-03-15") {
		t.Errorf("section should embed monitor link, got: %s", sectionText)
	}
	// Pipe must be escaped so it doesn't break Slack's <url|text> markup.
	if !strings.Contains(sectionText, "&#124;") {
		t.Errorf("section should escape '|' in monitor name, got: %s", sectionText)
	}
	// Without AppURL the legacy `*Name*` markup is used; with AppURL we wrap as a link.
	if strings.Contains(sectionText, "<https://warden.example.com/monitors/mon-abc?date=2026-03-15|*API &#124; Server*>") == false {
		t.Errorf("expected Slack link markup wrapping bolded escaped name, got: %s", sectionText)
	}
}

// TestSlackDigest_NoAppURL_PreservesLegacyRendering ensures the digest still renders cleanly
// (plain bold names, no link) when app_url is empty, so existing installs aren't disturbed.
func TestSlackDigest_NoAppURL_PreservesLegacyRendering(t *testing.T) {
	var received map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	notifier := NewSlackNotifier(`{"webhookUrl":"` + srv.URL + `"}`)
	summary := digestSummary{
		TotalEvents:  1,
		MonitorCount: 1,
		Monitors: []digestMonitor{
			{ID: "m", Name: "API", URL: "https://x", Events: []digestEventCount{{Type: "down", Count: 1}}, Severity: 0},
		},
		Date:   time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC),
		AppURL: "",
	}
	if err := notifier.sendDigest(summary); err != nil {
		t.Fatalf("sendDigest failed: %v", err)
	}

	ctxText := received["blocks"].([]interface{})[1].(map[string]interface{})["elements"].([]interface{})[0].(map[string]interface{})["text"].(string)
	if strings.Contains(ctxText, "View full report") {
		t.Errorf("no link expected when AppURL is empty, got: %s", ctxText)
	}
	sectionText := received["blocks"].([]interface{})[3].(map[string]interface{})["text"].(map[string]interface{})["text"].(string)
	if !strings.Contains(sectionText, "*API*") {
		t.Errorf("expected plain *API* with no AppURL, got: %s", sectionText)
	}
	if strings.Contains(sectionText, "<http") {
		t.Errorf("expected no Slack link markup with empty AppURL, got: %s", sectionText)
	}
}

func TestSlackDigest_SSLMessage(t *testing.T) {
	var received map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	config := `{"webhookUrl":"` + srv.URL + `"}`
	notifier := NewSlackNotifier(config)

	summary := digestSummary{
		TotalEvents:  1,
		MonitorCount: 1,
		Monitors: []digestMonitor{
			{
				Name:       "Google",
				URL:        "https://google.com",
				Events:     []digestEventCount{{Type: "ssl_expiring", Count: 1}},
				Severity:   3,
				SSLMessage: "SSL certificate expires in 14 days (2026-03-30)",
			},
		},
		Date: time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC),
	}

	if err := notifier.sendDigest(summary); err != nil {
		t.Fatalf("sendDigest failed: %v", err)
	}

	blocks := received["blocks"].([]interface{})
	section := blocks[3].(map[string]interface{})
	text := section["text"].(map[string]interface{})["text"].(string)

	// Should contain the actual SSL message, not the generic label
	if !strings.Contains(text, "SSL certificate expires in 14 days") {
		t.Errorf("expected SSL expiry message in section text, got: %s", text)
	}
	// Should NOT contain the generic count format
	if strings.Contains(text, "`1x`") {
		t.Errorf("should not contain generic count for SSL events when message exists, got: %s", text)
	}
}

func TestSlackDigest_SSLMessageFallback(t *testing.T) {
	var received map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	config := `{"webhookUrl":"` + srv.URL + `"}`
	notifier := NewSlackNotifier(config)

	summary := digestSummary{
		TotalEvents:  1,
		MonitorCount: 1,
		Monitors: []digestMonitor{
			{
				Name:       "Google",
				URL:        "https://google.com",
				Events:     []digestEventCount{{Type: "ssl_expiring", Count: 1}},
				Severity:   3,
				SSLMessage: "", // empty — should fall back to generic format
			},
		},
		Date: time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC),
	}

	if err := notifier.sendDigest(summary); err != nil {
		t.Fatalf("sendDigest failed: %v", err)
	}

	blocks := received["blocks"].([]interface{})
	section := blocks[3].(map[string]interface{})
	text := section["text"].(map[string]interface{})["text"].(string)

	if !strings.Contains(text, "SSL Expiring `1x`") {
		t.Errorf("expected generic count format when SSL message is empty, got: %s", text)
	}
}

func TestSlackDigest_MixedEventsWithSSL(t *testing.T) {
	var received map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	config := `{"webhookUrl":"` + srv.URL + `"}`
	notifier := NewSlackNotifier(config)

	summary := digestSummary{
		TotalEvents:  4,
		MonitorCount: 1,
		Monitors: []digestMonitor{
			{
				Name: "API Server",
				URL:  "https://api.example.com",
				Events: []digestEventCount{
					{Type: "down", Count: 2},
					{Type: "ssl_expiring", Count: 1},
					{Type: "up", Count: 1},
				},
				Severity:   0,
				SSLMessage: "SSL certificate expires in 7 days (2026-03-23)",
			},
		},
		Date: time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC),
	}

	if err := notifier.sendDigest(summary); err != nil {
		t.Fatalf("sendDigest failed: %v", err)
	}

	blocks := received["blocks"].([]interface{})
	section := blocks[3].(map[string]interface{})
	text := section["text"].(map[string]interface{})["text"].(string)

	// Down should use generic format
	if !strings.Contains(text, "Down `2x`") {
		t.Errorf("expected 'Down `2x`' in text, got: %s", text)
	}
	// SSL should use the actual message
	if !strings.Contains(text, "SSL certificate expires in 7 days") {
		t.Errorf("expected SSL message in text, got: %s", text)
	}
	// Up should use generic format
	if !strings.Contains(text, "Up `1x`") {
		t.Errorf("expected 'Up `1x`' in text, got: %s", text)
	}
}

func TestWebhookDigest_Payload(t *testing.T) {
	var received map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	config := `{"webhookUrl":"` + srv.URL + `"}`
	notifier := NewWebhookNotifier(config)

	summary := digestSummary{
		TotalEvents:  3,
		MonitorCount: 1,
		Monitors: []digestMonitor{
			{
				Name:     "API Server",
				URL:      "https://api.example.com",
				Events:   []digestEventCount{{Type: "down", Count: 2}, {Type: "up", Count: 1}},
				Severity: 0,
			},
		},
		Date: time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC),
	}

	events := []db.DigestEvent{
		{MonitorID: "m1", MonitorName: "API Server", EventType: "down"},
		{MonitorID: "m1", MonitorName: "API Server", EventType: "down"},
		{MonitorID: "m1", MonitorName: "API Server", EventType: "up"},
	}

	if err := notifier.sendDigest(summary, events); err != nil {
		t.Fatalf("sendDigest failed: %v", err)
	}

	// Verify all expected fields
	expectedFields := []string{"type", "title", "summary", "eventCount", "monitorCount", "monitors", "timestamp"}
	for _, field := range expectedFields {
		if _, ok := received[field]; !ok {
			t.Errorf("missing field %q in webhook digest payload", field)
		}
	}

	if received["type"] != "digest" {
		t.Errorf("expected type 'digest', got %v", received["type"])
	}
	if received["eventCount"].(float64) != 3 {
		t.Errorf("expected eventCount 3, got %v", received["eventCount"])
	}
	if received["monitorCount"].(float64) != 1 {
		t.Errorf("expected monitorCount 1, got %v", received["monitorCount"])
	}

	monitors := received["monitors"].([]interface{})
	if len(monitors) != 1 {
		t.Fatalf("expected 1 monitor, got %d", len(monitors))
	}

	mon := monitors[0].(map[string]interface{})
	if mon["name"] != "API Server" {
		t.Errorf("expected monitor name 'API Server', got %v", mon["name"])
	}
	if mon["url"] != "https://api.example.com" {
		t.Errorf("expected monitor URL, got %v", mon["url"])
	}
}

func TestWebhookDigest_SSLMessage(t *testing.T) {
	var received map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	config := `{"webhookUrl":"` + srv.URL + `"}`
	notifier := NewWebhookNotifier(config)

	summary := digestSummary{
		TotalEvents:  1,
		MonitorCount: 1,
		Monitors: []digestMonitor{
			{
				Name:       "Google",
				URL:        "https://google.com",
				Events:     []digestEventCount{{Type: "ssl_expiring", Count: 1}},
				Severity:   3,
				SSLMessage: "SSL certificate expires in 14 days (2026-03-30)",
			},
		},
		Date: time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC),
	}

	events := []db.DigestEvent{
		{MonitorID: "m1", MonitorName: "Google", EventType: "ssl_expiring",
			Message: "SSL certificate expires in 14 days (2026-03-30)"},
	}

	if err := notifier.sendDigest(summary, events); err != nil {
		t.Fatalf("sendDigest failed: %v", err)
	}

	summaryText := received["summary"].(string)
	if !strings.Contains(summaryText, "SSL certificate expires in 14 days") {
		t.Errorf("expected SSL message in summary, got: %s", summaryText)
	}
}

func TestSendDigest_SeveritySorting(t *testing.T) {
	store := newTestStore(t)
	svc := NewService(store)

	var received map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ch := db.NotificationChannel{
		ID:        "nc-test-sort",
		Type:      "slack",
		Name:      "Test Slack",
		Config:    `{"webhookUrl":"` + srv.URL + `"}`,
		Enabled:   true,
		CreatedAt: time.Now(),
	}
	if err := store.CreateNotificationChannel(ch); err != nil {
		t.Fatalf("Failed to create channel: %v", err)
	}

	// Input: stabilized first, down second — output should be sorted by severity
	events := []db.DigestEvent{
		{MonitorID: "m1", MonitorName: "Web App", MonitorURL: "https://app.example.com",
			EventType: "stabilized", Message: "Monitor stabilized", EventTime: time.Now()},
		{MonitorID: "m2", MonitorName: "API Server", MonitorURL: "https://api.example.com",
			EventType: "down", Message: "Connection refused", EventTime: time.Now()},
	}

	svc.SendDigest(events)

	blocks := received["blocks"].([]interface{})
	// blocks[3] = first monitor section (should be "down" = API Server)
	// blocks[4] = second monitor section (should be "stabilized" = Web App)
	section1 := blocks[3].(map[string]interface{})
	text1 := section1["text"].(map[string]interface{})["text"].(string)

	section2 := blocks[4].(map[string]interface{})
	text2 := section2["text"].(map[string]interface{})["text"].(string)

	if !strings.Contains(text1, "API Server") {
		t.Errorf("expected first monitor to be API Server (down), got: %s", text1)
	}
	if !strings.Contains(text2, "Web App") {
		t.Errorf("expected second monitor to be Web App (stabilized), got: %s", text2)
	}
}

func TestSendDigest_SSLMessageFromEvents(t *testing.T) {
	store := newTestStore(t)
	svc := NewService(store)

	var received map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ch := db.NotificationChannel{
		ID:        "nc-test-ssl",
		Type:      "slack",
		Name:      "Test Slack",
		Config:    `{"webhookUrl":"` + srv.URL + `"}`,
		Enabled:   true,
		CreatedAt: time.Now(),
	}
	if err := store.CreateNotificationChannel(ch); err != nil {
		t.Fatalf("Failed to create channel: %v", err)
	}

	events := []db.DigestEvent{
		{MonitorID: "m1", MonitorName: "Google", MonitorURL: "https://google.com",
			EventType: "ssl_expiring", Message: "SSL certificate expires in 14 days (2026-03-30)",
			EventTime: time.Now()},
	}

	svc.SendDigest(events)

	blocks := received["blocks"].([]interface{})
	section := blocks[3].(map[string]interface{})
	text := section["text"].(map[string]interface{})["text"].(string)

	if !strings.Contains(text, "SSL certificate expires in 14 days") {
		t.Errorf("expected SSL message from event in digest, got: %s", text)
	}
}
