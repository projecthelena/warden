package notifications

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestSlackAlertLayoutBoundsAndPlainText(t *testing.T) {
	event := sampleEvent()
	event.MonitorName = "<!channel> " + strings.Repeat("界", 400)
	event.Message = strings.Repeat("界", 5000)
	payload := slackAlertPayload(event)
	blocks := payload["attachments"].([]map[string]interface{})[0]["blocks"].([]map[string]interface{})
	if len(blocks) != 3 {
		t.Fatal("unexpected layout")
	}
	for i, limit := range []int{150, 3000} {
		text := blocks[i]["text"].(map[string]interface{})
		if text["type"] != "plain_text" {
			t.Fatal("untrusted alert text can trigger mentions")
		}
		if utf8.RuneCountInString(text["text"].(string)) > limit {
			t.Fatal("Slack text limit exceeded")
		}
	}
	if !strings.Contains(payload["text"].(string), "Monitor Down") {
		t.Fatal("missing accessible fallback")
	}
}

func TestSlackDigestBoundsLargeMonitorList(t *testing.T) {
	var payload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	config, _ := json.Marshal(map[string]string{"webhookUrl": server.URL})
	summary := digestSummary{TotalEvents: 1000, MonitorCount: 1000, Date: time.Now()}
	for i := 0; i < 1000; i++ {
		summary.Monitors = append(summary.Monitors, digestMonitor{Name: fmt.Sprintf("Service %d", i), Events: []digestEventCount{{Type: "down", Count: 1}}})
	}
	if err := NewSlackNotifier(string(config)).sendDigest(summary); err != nil {
		t.Fatal(err)
	}
	blocks := payload["blocks"].([]interface{})
	if len(blocks) != 12 {
		t.Fatalf("unbounded digest: %d blocks", len(blocks))
	}
	last := blocks[len(blocks)-1].(map[string]interface{})["text"].(map[string]interface{})["text"].(string)
	if !strings.Contains(last, "992 more") {
		t.Fatalf("hidden monitor count lost: %s", last)
	}
}

func TestSlackDeliveryErrorsDoNotExposeWebhookCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	target := server.URL + "/private-webhook-token"
	server.Close()
	for _, address := range []string{target, "https://example.invalid/%private-webhook-token"} {
		config, _ := json.Marshal(map[string]string{"webhookUrl": address})
		err := NewSlackNotifier(string(config)).Send(sampleEvent())
		if err == nil {
			t.Fatal("expected delivery error")
		}
		if strings.Contains(err.Error(), "private-webhook-token") || strings.Contains(err.Error(), address) {
			t.Fatalf("credential exposed: %v", err)
		}
	}
}
