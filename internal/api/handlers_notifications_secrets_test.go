package api

import (
	"encoding/json"
	"strings"
	"testing"
)

// A webhook URL is a credential: whoever holds it can post into the channel. A viewer
// can neither set nor test one, so there is no reason to hand it to them.
func TestMaskSecretsHidesCredentials(t *testing.T) {
	masked := maskSecrets(`{"webhookUrl":"https://hooks.slack.com/services/T000/B000/XXXX","channel":"#status"}`)

	if strings.Contains(masked, "hooks.slack.com") {
		t.Errorf("expected the webhook to be hidden, got %s", masked)
	}

	var fields map[string]any
	if err := json.Unmarshal([]byte(masked), &fields); err != nil {
		t.Fatalf("expected the masked config to stay valid JSON: %v", err)
	}
	if fields["webhookUrl"] != "***" {
		t.Errorf("expected the key to remain with a masked value, got %v", fields["webhookUrl"])
	}
}

// A channel type added later will store its secret under a name nobody listed here, so
// masking has to cover whatever it finds rather than a set of known names.
func TestMaskSecretsCoversFieldsNobodyAnticipated(t *testing.T) {
	masked := maskSecrets(`{"webhookUrl":"https://example.com/hook","botToken":"xoxb-secret","somethingNew":"also secret"}`)
	for _, secret := range []string{"example.com", "xoxb-secret", "also secret"} {
		if strings.Contains(masked, secret) {
			t.Errorf("expected %s to be hidden, got %s", secret, masked)
		}
	}
	// The keys stay, so the UI can still show which channel is which.
	if !strings.Contains(masked, "webhookUrl") {
		t.Errorf("expected the keys to survive, got %s", masked)
	}
}

// Unparseable config could hold anything, so say nothing rather than guess at it.
func TestMaskSecretsDropsConfigItCannotRead(t *testing.T) {
	if got := maskSecrets("not json at all"); got != "{}" {
		t.Errorf("expected an empty object, got %q", got)
	}
	if got := maskSecrets(""); got != "" {
		t.Errorf("expected an empty config to stay empty, got %q", got)
	}
}
