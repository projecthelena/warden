package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/clusteruptime/clusteruptime/internal/db"
)

// EventType defines the type of event that occurred
type EventType string

const (
	EventDown        EventType = "down"
	EventUp          EventType = "up"
	EventDegraded    EventType = "degraded"
	EventSSLExpiring EventType = "ssl_expiring"
)

// NotificationEvent represents the data needed to send a notification
type NotificationEvent struct {
	MonitorID   string
	MonitorName string
	MonitorURL  string
	Type        EventType
	Message     string
	Time        time.Time
}

// Notifier interfaces for different notification providers
type Notifier interface {
	Send(event NotificationEvent) error
}

// Service manages the notification queue and dispatching
type Service struct {
	store *db.Store
	queue chan NotificationEvent
}

func NewService(store *db.Store) *Service {
	return &Service{
		store: store,
		queue: make(chan NotificationEvent, 100),
	}
}

func (s *Service) Start() {
	go s.worker()
}

func (s *Service) worker() {
	for event := range s.queue {
		s.dispatch(event)
	}
}

func (s *Service) dispatch(event NotificationEvent) {
	channels, err := s.store.GetNotificationChannels()
	if err != nil {
		log.Printf("Failed to fetch notification channels: %v", err)
		return
	}

	for _, ch := range channels {
		if !ch.Enabled {
			continue
		}

		var notifier Notifier
		switch ch.Type {
		case "slack":
			notifier = NewSlackNotifier(ch.Config)
		// Add other types here (email, etc.)
		default:
			log.Printf("Unknown channel type: %s", ch.Type)
			continue
		}

		if err := notifier.Send(event); err != nil {
			log.Printf("Failed to send notification to %s (%s): %v", ch.Name, ch.Type, err)
		}
	}
}

func (s *Service) Enqueue(event NotificationEvent) {
	select {
	case s.queue <- event:
	default:
		log.Printf("Notification queue full, dropping event for %s", event.MonitorID)
	}
}

// SlackNotifier implementation
type SlackNotifier struct {
	config map[string]interface{}
}

func NewSlackNotifier(configJSON string) *SlackNotifier {
	var config map[string]interface{}
	_ = json.Unmarshal([]byte(configJSON), &config)
	return &SlackNotifier{config: config}
}

func (n *SlackNotifier) Send(event NotificationEvent) error {
	url, ok := n.config["webhookUrl"].(string)
	if !ok || url == "" {
		return fmt.Errorf("webhookUrl missing or invalid")
	}

	color := "#36a64f" // Green (Up)
	switch event.Type {
	case EventDown:
		color = "#dc3545" // Red
	case EventDegraded:
		color = "#ffc107" // Yellow
	case EventSSLExpiring:
		color = "#ff8c00" // Orange
	}

	emoji := ":white_check_mark:"
	switch event.Type {
	case EventDown:
		emoji = ":rotating_light:"
	case EventDegraded:
		emoji = ":warning:"
	case EventSSLExpiring:
		emoji = ":lock:"
	}

	title := "Monitor Recovered"
	switch event.Type {
	case EventDown:
		title = "Monitor Down"
	case EventDegraded:
		title = "Monitor Degraded"
	case EventSSLExpiring:
		title = "SSL Certificate Expiring"
	}

	payload := map[string]interface{}{
		"text": "*" + title + "*: " + event.MonitorName,
		"attachments": []map[string]interface{}{
			{
				"color": color,
				"fields": []map[string]interface{}{
					{
						"title": "Monitor",
						"value": event.MonitorName,
						"short": true,
					},
					{
						"title": "URL",
						"value": event.MonitorURL,
						"short": true,
					},
					{
						"title": "Message",
						"value": emoji + " " + event.Message,
						"short": false,
					},
					{
						"title": "Time",
						"value": event.Time.Format(time.RFC1123),
						"short": true,
					},
				},
			},
		},
	}

	return sendJSON(url, payload)
}

func sendJSON(url string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("request failed with status code %d", resp.StatusCode)
	}

	return nil
}
