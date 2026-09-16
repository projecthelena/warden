package notifications

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTelegramNotifierSendsMessage(t *testing.T) {
	var payload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bot123456:test-token/sendMessage" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewTelegramNotifier(`{"botToken":"123456:test-token","chatId":"-100123"}`)
	notifier.apiURL = server.URL
	err := notifier.Send(NotificationEvent{MonitorName: "API <prod>", MonitorURL: "https://example.com", Type: EventDown, Message: "connection refused", Time: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if payload["chat_id"] != "-100123" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if payload["text"] == "" {
		t.Fatal("message text is empty")
	}
}

func TestTelegramNotifierSplitsLongMessages(t *testing.T) {
	var lengths []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		lengths = append(lengths, len([]rune(payload["text"].(string))))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewTelegramNotifier(`{"botToken":"123456:test-token","chatId":"123"}`)
	notifier.apiURL = server.URL
	if err := notifier.Send(NotificationEvent{Message: strings.Repeat("á", telegramMessageLimit*2), Time: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if len(lengths) != 3 {
		t.Fatalf("expected 3 Telegram messages, got %d", len(lengths))
	}
	for _, length := range lengths {
		if length > telegramMessageLimit {
			t.Fatalf("message exceeded Telegram limit: %d", length)
		}
	}
}

func TestTelegramSendDirect(t *testing.T) {
	// SendDirect uses the production API URL, so the chunking and HTTP request are covered
	// through TelegramNotifier while this test guards the provider registration.
	err := SendDirect("telegram", `{}`, NotificationEvent{})
	if err == nil || !strings.Contains(err.Error(), "botToken") {
		t.Fatalf("Telegram provider is not registered: %v", err)
	}
}

func TestTelegramNotifierRequiresCredentials(t *testing.T) {
	if err := NewTelegramNotifier(`{}`).Send(NotificationEvent{}); err == nil {
		t.Fatal("expected missing credentials error")
	}
}

func TestTelegramNotifierRedactsTokenFromErrors(t *testing.T) {
	const token = "secret-bot-token"
	notifier := NewTelegramNotifier(`{"botToken":"` + token + `","chatId":"123"}`)
	notifier.apiURL = "://invalid"

	err := notifier.Send(NotificationEvent{})
	if err == nil {
		t.Fatal("expected request error")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("error exposed bot token: %v", err)
	}
}
