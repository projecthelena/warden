package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/projecthelena/warden/internal/db"
)

func newTestStore(t *testing.T) *db.Store {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	return store
}

func TestGetChannels(t *testing.T) {
	store := newTestStore(t)
	handler := NewNotificationChannelsHandler(store)

	channel := db.NotificationChannel{
		ID:        "nc1",
		Type:      "slack",
		Name:      "Test",
		Config:    "{}",
		Enabled:   true,
		CreatedAt: time.Now(),
	}
	if err := store.CreateNotificationChannel(channel); err != nil {
		t.Fatalf("Failed to create channel: %v", err)
	}

	req, _ := http.NewRequest("GET", "/api/notifications/channels", nil)
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUserRole, RoleAdmin))
	rr := httptest.NewRecorder()

	handler.GetChannels(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	var response map[string][]db.NotificationChannel
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if len(response["channels"]) != 1 {
		t.Errorf("expected 1 channel, got %d", len(response["channels"]))
	}
}

func testAdminRoleMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), contextKeyUserRole, RoleAdmin)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func TestCreateChannel(t *testing.T) {
	store := newTestStore(t)
	handler := NewNotificationChannelsHandler(store)

	payload := map[string]interface{}{
		"type":    "slack",
		"name":    "My Slack",
		"config":  map[string]string{"webhookUrl": "http://example.com"},
		"enabled": true,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "/api/notifications/channels", bytes.NewBuffer(body))
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUserRole, RoleAdmin))
	rr := httptest.NewRecorder()

	handler.CreateChannel(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusCreated)
	}

	channels, _ := store.GetNotificationChannels()
	if len(channels) != 1 {
		t.Errorf("expected 1 channel in db, got %d", len(channels))
	}
	if channels[0].Name != "My Slack" {
		t.Errorf("expected name 'My Slack', got '%s'", channels[0].Name)
	}
}

func TestDeleteChannel(t *testing.T) {
	store := newTestStore(t)
	handler := NewNotificationChannelsHandler(store)
	if err := store.CreateNotificationChannel(db.NotificationChannel{ID: "nc1", Type: "slack", Name: "To Delete", Config: "{}", Enabled: true}); err != nil {
		t.Fatalf("Failed to create channel: %v", err)
	}

	// Setup CHI router to handle params
	r := chi.NewRouter()
	r.Use(testAdminRoleMiddleware)
	r.Delete("/notifications/channels/{id}", handler.DeleteChannel)

	req, _ := http.NewRequest("DELETE", "/notifications/channels/nc1", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}

	channels, _ := store.GetNotificationChannels()
	if len(channels) != 0 {
		t.Errorf("expected 0 channels, got %d", len(channels))
	}
}

func TestUpdateChannel(t *testing.T) {
	store := newTestStore(t)
	handler := NewNotificationChannelsHandler(store)

	// Seed a channel
	if err := store.CreateNotificationChannel(db.NotificationChannel{
		ID: "nc1", Type: "slack", Name: "Old Name",
		Config: `{"webhookUrl":"http://old.example.com"}`, Enabled: true,
	}); err != nil {
		t.Fatalf("Failed to create channel: %v", err)
	}

	r := chi.NewRouter()
	r.Use(testAdminRoleMiddleware)
	r.Put("/notifications/channels/{id}", handler.UpdateChannel)

	payload := map[string]interface{}{
		"type":    "webhook",
		"name":    "New Name",
		"config":  map[string]string{"webhookUrl": "http://new.example.com/hook"},
		"enabled": true,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PUT", "/notifications/channels/nc1", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	channels, _ := store.GetNotificationChannels()
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}
	if channels[0].Name != "New Name" {
		t.Errorf("expected name 'New Name', got '%s'", channels[0].Name)
	}
	if channels[0].Type != "webhook" {
		t.Errorf("expected type 'webhook', got '%s'", channels[0].Type)
	}
	if !channels[0].Enabled {
		t.Error("expected the channel to remain enabled")
	}
}

func TestUpdateChannel_PreservesEnabledWhenOmitted(t *testing.T) {
	store := newTestStore(t)
	handler := NewNotificationChannelsHandler(store)

	if err := store.CreateNotificationChannel(db.NotificationChannel{
		ID: "nc-disabled", Type: "webhook", Name: "Disabled",
		Config: `{"webhookUrl":"http://old.example.com"}`, Enabled: false,
	}); err != nil {
		t.Fatalf("Failed to create channel: %v", err)
	}

	r := chi.NewRouter()
	r.Use(testAdminRoleMiddleware)
	r.Put("/notifications/channels/{id}", handler.UpdateChannel)

	payload := map[string]interface{}{
		"type":   "webhook",
		"name":   "Still Disabled",
		"config": map[string]string{"webhookUrl": "http://new.example.com/hook"},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PUT", "/notifications/channels/nc-disabled", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	channels, err := store.GetNotificationChannels()
	if err != nil {
		t.Fatalf("GetNotificationChannels: %v", err)
	}
	if len(channels) != 1 || channels[0].Enabled {
		t.Fatalf("an update without enabled changed the channel state: %+v", channels)
	}
}

func TestUpdateChannel_ValidationErrors(t *testing.T) {
	store := newTestStore(t)
	handler := NewNotificationChannelsHandler(store)

	if err := store.CreateNotificationChannel(db.NotificationChannel{
		ID: "nc1", Type: "slack", Name: "Test",
		Config: `{"webhookUrl":"http://example.com"}`, Enabled: true,
	}); err != nil {
		t.Fatalf("Failed to create channel: %v", err)
	}

	r := chi.NewRouter()
	r.Use(testAdminRoleMiddleware)
	r.Put("/notifications/channels/{id}", handler.UpdateChannel)

	tests := []struct {
		name       string
		payload    map[string]interface{}
		wantStatus int
	}{
		{
			name:       "missing type",
			payload:    map[string]interface{}{"name": "Test", "config": map[string]string{"webhookUrl": "http://example.com"}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing name",
			payload:    map[string]interface{}{"type": "slack", "config": map[string]string{"webhookUrl": "http://example.com"}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid URL scheme",
			payload:    map[string]interface{}{"type": "slack", "name": "Test", "config": map[string]string{"webhookUrl": "ftp://example.com"}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty webhook URL",
			payload:    map[string]interface{}{"type": "webhook", "name": "Test", "config": map[string]string{"webhookUrl": ""}},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.payload)
			req, _ := http.NewRequest("PUT", "/notifications/channels/nc1", bytes.NewBuffer(body))
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("expected %d, got %d: %s", tc.wantStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestCreateChannel_Webhook(t *testing.T) {
	store := newTestStore(t)
	handler := NewNotificationChannelsHandler(store)

	payload := map[string]interface{}{
		"type":    "webhook",
		"name":    "My Webhook",
		"config":  map[string]string{"webhookUrl": "https://my-api.com/webhook"},
		"enabled": true,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "/api/notifications/channels", bytes.NewBuffer(body))
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUserRole, RoleAdmin))
	rr := httptest.NewRecorder()

	handler.CreateChannel(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	channels, _ := store.GetNotificationChannels()
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}
	if channels[0].Type != "webhook" {
		t.Errorf("expected type 'webhook', got '%s'", channels[0].Type)
	}
}

func TestCreateChannel_WebhookValidation(t *testing.T) {
	store := newTestStore(t)
	handler := NewNotificationChannelsHandler(store)

	tests := []struct {
		name       string
		payload    map[string]interface{}
		wantStatus int
	}{
		{
			name: "missing webhook URL",
			payload: map[string]interface{}{
				"type": "webhook", "name": "Test",
				"config": map[string]string{},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid scheme",
			payload: map[string]interface{}{
				"type": "webhook", "name": "Test",
				"config": map[string]string{"webhookUrl": "ftp://bad.example.com"},
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.payload)
			req, _ := http.NewRequest("POST", "/api/notifications/channels", bytes.NewBuffer(body))
			req = req.WithContext(context.WithValue(req.Context(), contextKeyUserRole, RoleAdmin))
			rr := httptest.NewRecorder()
			handler.CreateChannel(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("expected %d, got %d: %s", tc.wantStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestTestChannel_Success(t *testing.T) {
	// Spin up a fake webhook receiver
	received := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store := newTestStore(t)
	handler := NewNotificationChannelsHandler(store)

	payload := map[string]interface{}{
		"type":   "webhook",
		"config": map[string]string{"webhookUrl": srv.URL},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "/api/notifications/channels/test", bytes.NewBuffer(body))
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUserRole, RoleAdmin))
	rr := httptest.NewRecorder()

	handler.TestChannel(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if !received {
		t.Error("test webhook endpoint was not called")
	}
}

func TestTestChannel_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	store := newTestStore(t)
	handler := NewNotificationChannelsHandler(store)

	payload := map[string]interface{}{
		"type":   "webhook",
		"config": map[string]string{"webhookUrl": srv.URL},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "/api/notifications/channels/test", bytes.NewBuffer(body))
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUserRole, RoleAdmin))
	rr := httptest.NewRecorder()

	handler.TestChannel(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("expected 502 for upstream error, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestTestChannel_MissingType(t *testing.T) {
	store := newTestStore(t)
	handler := NewNotificationChannelsHandler(store)

	payload := map[string]interface{}{
		"config": map[string]string{"webhookUrl": "http://example.com"},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "/api/notifications/channels/test", bytes.NewBuffer(body))
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUserRole, RoleAdmin))
	rr := httptest.NewRecorder()

	handler.TestChannel(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestTestChannel_InvalidURL(t *testing.T) {
	store := newTestStore(t)
	handler := NewNotificationChannelsHandler(store)

	payload := map[string]interface{}{
		"type":   "webhook",
		"config": map[string]string{"webhookUrl": "not-a-url"},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "/api/notifications/channels/test", bytes.NewBuffer(body))
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUserRole, RoleAdmin))
	rr := httptest.NewRecorder()

	handler.TestChannel(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateChannel_Email(t *testing.T) {
	store := newTestStore(t)
	handler := NewNotificationChannelsHandler(store)

	payload := map[string]interface{}{
		"type": "email", "name": "On-call inbox",
		"config": map[string]string{
			"host": "smtp.example.com",
			"port": "587",
			"from": "Warden <alerts@example.com>",
			"to":   "oncall@example.com, cto@example.com",
		},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/notifications/channels", bytes.NewBuffer(body))
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUserRole, RoleAdmin))
	rr := httptest.NewRecorder()

	handler.CreateChannel(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	channels, err := store.GetNotificationChannels()
	if err != nil {
		t.Fatalf("GetNotificationChannels: %v", err)
	}
	if len(channels) != 1 || channels[0].Type != "email" {
		t.Fatalf("expected one email channel, got %+v", channels)
	}
}

// An email channel has no webhook URL. Before email existed the handler validated one for
// every type, which would have rejected this outright.
func TestCreateChannel_EmailValidation(t *testing.T) {
	store := newTestStore(t)
	handler := NewNotificationChannelsHandler(store)

	tests := []struct {
		name       string
		config     map[string]string
		wantStatus int
	}{
		{
			name:       "missing server",
			config:     map[string]string{"from": "a@example.com", "to": "b@example.com"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing recipients",
			config:     map[string]string{"host": "smtp.example.com", "from": "a@example.com"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "malformed recipient",
			config:     map[string]string{"host": "smtp.example.com", "from": "a@example.com", "to": "not-an-address"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "valid without credentials",
			config:     map[string]string{"host": "smtp.example.com", "from": "a@example.com", "to": "b@example.com"},
			wantStatus: http.StatusCreated,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]interface{}{"type": "email", "name": "Mail", "config": tc.config})
			req, _ := http.NewRequest("POST", "/api/notifications/channels", bytes.NewBuffer(body))
			req = req.WithContext(context.WithValue(req.Context(), contextKeyUserRole, RoleAdmin))
			rr := httptest.NewRecorder()
			handler.CreateChannel(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("expected %d, got %d: %s", tc.wantStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestTestChannel_EmailRejectsBrokenConfig(t *testing.T) {
	store := newTestStore(t)
	handler := NewNotificationChannelsHandler(store)

	body, _ := json.Marshal(map[string]interface{}{
		"type":   "email",
		"config": map[string]string{"host": "smtp.example.com", "from": "a@example.com"},
	})
	req, _ := http.NewRequest("POST", "/api/notifications/channels/test", bytes.NewBuffer(body))
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUserRole, RoleAdmin))
	rr := httptest.NewRecorder()

	handler.TestChannel(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for a channel with no recipients, got %d: %s", rr.Code, rr.Body.String())
	}
}

// The dashboard reads a rejection as JSON and shows data.error. When the body is plain
// text res.json() rejects, data.error is undefined, and every reason — whichever field is
// wrong, and why — collapses into a generic "Failed to add channel." Validating per field
// is only worth anything if the reason survives the trip, so the shape is asserted here
// rather than the status alone.
func TestChannelValidation_ReportsTheReasonAsJSON(t *testing.T) {
	store := newTestStore(t)
	handler := NewNotificationChannelsHandler(store)

	if err := store.CreateNotificationChannel(db.NotificationChannel{
		ID: "nc1", Type: "email", Name: "Mail",
		Config: `{"host":"smtp.example.com","from":"a@example.com","to":"b@example.com"}`, Enabled: true,
	}); err != nil {
		t.Fatalf("seeding a channel: %v", err)
	}

	r := chi.NewRouter()
	r.Use(testAdminRoleMiddleware)
	r.Post("/notifications/channels", handler.CreateChannel)
	r.Put("/notifications/channels/{id}", handler.UpdateChannel)
	r.Post("/notifications/channels/test", handler.TestChannel)

	cases := []struct {
		name     string
		method   string
		path     string
		config   map[string]string
		wantSaid string
	}{
		{
			name:     "create without a recipient",
			method:   "POST",
			path:     "/notifications/channels",
			config:   map[string]string{"host": "smtp.example.com", "from": "a@example.com"},
			wantSaid: "at least one recipient is required",
		},
		{
			name:     "create with a malformed recipient",
			method:   "POST",
			path:     "/notifications/channels",
			config:   map[string]string{"host": "smtp.example.com", "from": "a@example.com", "to": "nope"},
			wantSaid: "nope",
		},
		{
			name:     "create with an impossible port",
			method:   "POST",
			path:     "/notifications/channels",
			config:   map[string]string{"host": "smtp.example.com", "port": "70000", "from": "a@example.com", "to": "b@example.com"},
			wantSaid: "70000",
		},
		{
			name:     "update without a server",
			method:   "PUT",
			path:     "/notifications/channels/nc1",
			config:   map[string]string{"from": "a@example.com", "to": "b@example.com"},
			wantSaid: "host is required",
		},
		{
			name:     "test without a recipient",
			method:   "POST",
			path:     "/notifications/channels/test",
			config:   map[string]string{"host": "smtp.example.com", "from": "a@example.com"},
			wantSaid: "at least one recipient is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]interface{}{"type": "email", "name": "Mail", "config": tc.config})
			req, _ := http.NewRequest(tc.method, tc.path, bytes.NewBuffer(body))
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
			}

			var decoded map[string]string
			if err := json.Unmarshal(rr.Body.Bytes(), &decoded); err != nil {
				t.Fatalf("the body is not JSON, so the dashboard shows a generic error: %q", rr.Body.String())
			}
			if !strings.Contains(decoded["error"], tc.wantSaid) {
				t.Errorf("expected the reason to mention %q, got %q", tc.wantSaid, decoded["error"])
			}
		})
	}

	// A rejected update must leave the stored channel exactly as it was.
	channels, _ := store.GetNotificationChannels()
	if len(channels) != 1 {
		t.Fatalf("expected the seeded channel and nothing else, got %d", len(channels))
	}
	if !strings.Contains(channels[0].Config, "smtp.example.com") {
		t.Errorf("the rejected update changed the stored config: %s", channels[0].Config)
	}
}
