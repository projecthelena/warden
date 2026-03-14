package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

// EventType defines the type of event that occurred
type EventType string

const (
	EventDown        EventType = "down"
	EventUp          EventType = "up"
	EventDegraded    EventType = "degraded"
	EventSSLExpiring EventType = "ssl_expiring"
	EventFlapping    EventType = "flapping"
	EventStabilized  EventType = "stabilized"
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
		case "webhook":
			notifier = NewWebhookNotifier(ch.Config)
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
	case EventFlapping:
		color = "#9b59b6" // Purple
	case EventStabilized:
		color = "#3498db" // Blue
	}

	emoji := ":white_check_mark:"
	switch event.Type {
	case EventDown:
		emoji = ":rotating_light:"
	case EventDegraded:
		emoji = ":warning:"
	case EventSSLExpiring:
		emoji = ":lock:"
	case EventFlapping:
		emoji = ":cyclone:"
	case EventStabilized:
		emoji = ":large_blue_circle:"
	}

	title := "Monitor Recovered"
	switch event.Type {
	case EventDown:
		title = "Monitor Down"
	case EventDegraded:
		title = "Monitor Degraded"
	case EventSSLExpiring:
		title = "SSL Certificate Expiring"
	case EventFlapping:
		title = "Monitor Flapping"
	case EventStabilized:
		title = "Monitor Stabilized"
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

// WebhookNotifier sends a clean JSON payload to a generic webhook endpoint
type WebhookNotifier struct {
	config map[string]interface{}
}

func NewWebhookNotifier(configJSON string) *WebhookNotifier {
	var config map[string]interface{}
	_ = json.Unmarshal([]byte(configJSON), &config)
	return &WebhookNotifier{config: config}
}

func (n *WebhookNotifier) Send(event NotificationEvent) error {
	webhookURL, ok := n.config["webhookUrl"].(string)
	if !ok || webhookURL == "" {
		return fmt.Errorf("webhookUrl missing or invalid")
	}

	payload := map[string]interface{}{
		"event":       string(event.Type),
		"monitorId":   event.MonitorID,
		"monitorName": event.MonitorName,
		"monitorUrl":  event.MonitorURL,
		"message":     event.Message,
		"timestamp":   event.Time.Format(time.RFC3339),
	}

	return sendJSON(webhookURL, payload)
}

// SendDirect dispatches a NotificationEvent through the appropriate notifier
// without going through the queue. Used for test notifications.
func SendDirect(channelType, configJSON string, event NotificationEvent) error {
	var notifier Notifier
	switch channelType {
	case "slack":
		notifier = NewSlackNotifier(configJSON)
	case "webhook":
		notifier = NewWebhookNotifier(configJSON)
	default:
		return fmt.Errorf("unsupported channel type: %s", channelType)
	}
	return notifier.Send(event)
}

// SendDigest dispatches a daily digest summary to all enabled notification channels.
func (s *Service) SendDigest(events []db.DigestEvent) {
	if len(events) == 0 {
		return
	}

	channels, err := s.store.GetNotificationChannels()
	if err != nil {
		log.Printf("Digest: failed to fetch channels: %v", err)
		return
	}

	// Group events by monitor
	type monitorEvents struct {
		name   string
		counts map[string]int
	}
	byMonitor := make(map[string]*monitorEvents)
	var monitorOrder []string

	for _, e := range events {
		me, ok := byMonitor[e.MonitorID]
		if !ok {
			me = &monitorEvents{name: e.MonitorName, counts: make(map[string]int)}
			byMonitor[e.MonitorID] = me
			monitorOrder = append(monitorOrder, e.MonitorID)
		}
		me.counts[e.EventType]++
	}

	// Build summary lines
	var lines []string
	for _, mid := range monitorOrder {
		me := byMonitor[mid]
		var parts []string
		// Sort event types for consistent output
		var types []string
		for t := range me.counts {
			types = append(types, t)
		}
		sort.Strings(types)
		for _, t := range types {
			parts = append(parts, t+" ("+strconv.Itoa(me.counts[t])+"x)")
		}
		lines = append(lines, "- "+me.name+": "+strings.Join(parts, ", "))
	}

	title := "Daily Monitoring Summary (" + strconv.Itoa(len(events)) + " events)"
	body := strings.Join(lines, "\n")

	for _, ch := range channels {
		if !ch.Enabled {
			continue
		}

		switch ch.Type {
		case "slack":
			n := NewSlackNotifier(ch.Config)
			if err := n.sendDigest(title, body); err != nil {
				log.Printf("Digest: failed to send to Slack (%s): %v", ch.Name, err)
			}
		case "webhook":
			n := NewWebhookNotifier(ch.Config)
			if err := n.sendDigest(title, body, events); err != nil {
				log.Printf("Digest: failed to send to webhook (%s): %v", ch.Name, err)
			}
		}
	}
}

func (n *SlackNotifier) sendDigest(title, body string) error {
	webhookURL, ok := n.config["webhookUrl"].(string)
	if !ok || webhookURL == "" {
		return fmt.Errorf("webhookUrl missing or invalid")
	}

	payload := map[string]interface{}{
		"text": ":bar_chart: *" + title + "*",
		"attachments": []map[string]interface{}{
			{
				"color": "#3498db",
				"text":  body,
			},
		},
	}

	return sendJSON(webhookURL, payload)
}

func (n *WebhookNotifier) sendDigest(title, body string, events []db.DigestEvent) error {
	webhookURL, ok := n.config["webhookUrl"].(string)
	if !ok || webhookURL == "" {
		return fmt.Errorf("webhookUrl missing or invalid")
	}

	payload := map[string]interface{}{
		"type":       "digest",
		"title":      title,
		"summary":    body,
		"eventCount": len(events),
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	return sendJSON(webhookURL, payload)
}

func sendJSON(targetURL string, payload interface{}) error {
	// SECURITY: Validate URL scheme to prevent SSRF if database is compromised
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return fmt.Errorf("invalid webhook URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid webhook URL scheme: %s", parsedURL.Scheme)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", targetURL, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req) // #nosec G704 -- URL scheme validated above
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("request failed with status code %d", resp.StatusCode)
	}

	return nil
}
