package notifications

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestDiscordNotifierSendsEmbed(t *testing.T) {
	var payload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	notifier := NewDiscordNotifier(`{"webhookUrl":"` + server.URL + `"}`)
	err := notifier.Send(NotificationEvent{MonitorName: "API", MonitorURL: "https://example.com", Type: EventDown, Message: "connection refused", Time: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if payload["username"] != "Warden" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	allowedMentions, ok := payload["allowed_mentions"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing allowed_mentions: %#v", payload)
	}
	parse, ok := allowedMentions["parse"].([]interface{})
	if !ok || len(parse) != 0 {
		t.Fatalf("Discord mentions must be disabled: %#v", allowedMentions)
	}
	embeds, ok := payload["embeds"].([]interface{})
	if !ok || len(embeds) != 1 {
		t.Fatalf("expected one Discord embed: %#v", payload)
	}
}

func TestDiscordNotifierTruncatesProviderFields(t *testing.T) {
	var payload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	notifier := NewDiscordNotifier(`{"webhookUrl":"` + server.URL + `"}`)
	err := notifier.Send(NotificationEvent{MonitorName: strings.Repeat("m", 300), MonitorURL: strings.Repeat("u", 1100), Type: EventDown, Message: strings.Repeat("x", 5000), Time: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	embed := payload["embeds"].([]interface{})[0].(map[string]interface{})
	if utf8.RuneCountInString(embed["title"].(string)) != discordTitleLimit {
		t.Fatalf("title was not truncated: %d", utf8.RuneCountInString(embed["title"].(string)))
	}
	if utf8.RuneCountInString(embed["description"].(string)) != discordDescriptionLimit {
		t.Fatalf("description was not truncated: %d", utf8.RuneCountInString(embed["description"].(string)))
	}
}

func TestDiscordNotifierRedactsWebhookFromErrors(t *testing.T) {
	const webhook = "https://127.0.0.1:1/api/webhooks/123/secret-token"
	err := NewDiscordNotifier(`{"webhookUrl":"` + webhook + `"}`).Send(NotificationEvent{})
	if err == nil {
		t.Fatal("expected request error")
	}
	if strings.Contains(err.Error(), webhook) || strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("error exposed Discord webhook: %v", err)
	}
}

func TestDiscordNotifierRequiresWebhook(t *testing.T) {
	if err := NewDiscordNotifier(`{}`).Send(NotificationEvent{}); err == nil {
		t.Fatal("expected missing webhook error")
	}
}

func TestDiscordSendDirect(t *testing.T) {
	received := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	if err := SendDirect("discord", `{"webhookUrl":"`+server.URL+`"}`, NotificationEvent{Type: EventDown, Time: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if !received {
		t.Fatal("Discord webhook was not called")
	}
}

func TestDiscordDigestTruncatesDescription(t *testing.T) {
	var payload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	monitors := make([]digestMonitor, 100)
	for i := range monitors {
		monitors[i] = digestMonitor{Name: strings.Repeat("monitor", 10), Events: []digestEventCount{{Type: string(EventDown), Count: 1}}}
	}
	notifier := NewDiscordNotifier(`{"webhookUrl":"` + server.URL + `"}`)
	if err := notifier.sendDigest(digestSummary{TotalEvents: 100, MonitorCount: 100, Monitors: monitors, Date: time.Now()}); err != nil {
		t.Fatal(err)
	}
	embed := payload["embeds"].([]interface{})[0].(map[string]interface{})
	if utf8.RuneCountInString(embed["description"].(string)) != discordDescriptionLimit {
		t.Fatalf("digest description was not truncated: %d", utf8.RuneCountInString(embed["description"].(string)))
	}
}
