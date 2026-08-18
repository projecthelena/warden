package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
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

// The unit tests above cover the masking. This covers the boundary: that the handler
// applies it by role. Without this, deleting the role check leaves every test passing.
func TestGetChannelsMasksForViewersOnly(t *testing.T) {
	store := newTestStore(t)
	handler := NewNotificationChannelsHandler(store)

	if err := store.CreateNotificationChannel(db.NotificationChannel{
		ID:        "nc1",
		Type:      "slack",
		Name:      "Alerts",
		Config:    `{"webhookUrl":"https://hooks.slack.com/services/T000/B000/SECRET"}`,
		Enabled:   true,
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	read := func(role string) string {
		req := httptest.NewRequest("GET", "/api/notifications/channels", nil)
		req = req.WithContext(context.WithValue(req.Context(), contextKeyUserRole, role))
		rr := httptest.NewRecorder()
		handler.GetChannels(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", role, rr.Code)
		}
		return rr.Body.String()
	}

	for _, role := range []string{RoleViewer, RoleStatusViewer} {
		if strings.Contains(read(role), "SECRET") {
			t.Errorf("%s must not be able to read the webhook", role)
		}
	}
	for _, role := range []string{RoleEditor, RoleAdmin} {
		if !strings.Contains(read(role), "SECRET") {
			t.Errorf("%s edits and tests channels, so it still needs the webhook", role)
		}
	}
}

// An SMTP password is a credential in a way a webhook URL is not: it is often the
// operator's real mailbox password, reused elsewhere. maskSecrets blanks every string
// rather than a list of known key names precisely so a channel type added later is covered
// without anyone remembering to update a list, and email is the first type to test that
// promise.
func TestGetChannelsMasksTheSMTPPassword(t *testing.T) {
	store := newTestStore(t)
	handler := NewNotificationChannelsHandler(store)

	if err := store.CreateNotificationChannel(db.NotificationChannel{
		ID:   "nc-email",
		Type: "email",
		Name: "Ops mailbox",
		Config: `{"host":"smtp.example.com","port":"587","username":"alerts@example.com",` +
			`"password":"SUPERSECRET","from":"alerts@example.com","to":"ops@example.com"}`,
		Enabled:   true,
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	read := func(role string) string {
		req := httptest.NewRequest("GET", "/api/notifications/channels", nil)
		req = req.WithContext(context.WithValue(req.Context(), contextKeyUserRole, role))
		rr := httptest.NewRecorder()
		handler.GetChannels(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", role, rr.Code)
		}
		return rr.Body.String()
	}

	for _, role := range []string{RoleViewer, RoleStatusViewer} {
		body := read(role)
		if strings.Contains(body, "SUPERSECRET") {
			t.Errorf("%s must not be able to read the SMTP password", role)
		}
		// The username is a credential half too, and the host tells an attacker where to
		// try it. Everything is masked, so neither should survive.
		if strings.Contains(body, "smtp.example.com") {
			t.Errorf("%s should not see the SMTP host either", role)
		}
	}

	for _, role := range []string{RoleEditor, RoleAdmin} {
		if !strings.Contains(read(role), "SUPERSECRET") {
			t.Errorf("%s edits and tests channels, so the edit form still needs the password", role)
		}
	}
}

// Masking has to survive a config whose values are not all strings: the port arrives as a
// JSON number from the form, and a mask that choked on it would drop the whole config and
// take the password's key with it.
func TestMaskSecretsHandlesANumericPort(t *testing.T) {
	masked := maskSecrets(`{"host":"smtp.example.com","port":587,"password":"SUPERSECRET"}`)

	if strings.Contains(masked, "SUPERSECRET") {
		t.Fatalf("the password survived masking: %s", masked)
	}

	var fields map[string]any
	if err := json.Unmarshal([]byte(masked), &fields); err != nil {
		t.Fatalf("masked config is not valid JSON: %v", err)
	}
	if fields["password"] != "***" {
		t.Errorf("password = %v, want ***", fields["password"])
	}
	// A port is not a secret, and keeping it lets the UI tell one channel from another.
	if fields["port"] != float64(587) {
		t.Errorf("port = %v, want it left alone", fields["port"])
	}
}
