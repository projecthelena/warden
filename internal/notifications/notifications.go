package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
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

// digestMonitor holds the summary data for one monitor in the digest.
type digestMonitor struct {
	Name       string
	URL        string
	Events     []digestEventCount
	Severity   int    // worst event severity (lower = more severe)
	SSLMessage string // latest SSL expiry message (e.g. "SSL certificate expires in 14 days (2026-03-30)")
}

// digestEventCount holds the count for one event type.
type digestEventCount struct {
	Type  string
	Count int
}

// digestSummary holds the full digest data.
type digestSummary struct {
	TotalEvents  int
	MonitorCount int
	Monitors     []digestMonitor
	Date         time.Time
}

func eventSeverity(eventType string) int {
	switch eventType {
	case "down":
		return 0
	case "degraded":
		return 1
	case "flapping":
		return 2
	case "ssl_expiring":
		return 3
	case "stabilized":
		return 4
	case "up":
		return 5
	default:
		return 6
	}
}

func eventEmoji(eventType string) string {
	switch eventType {
	case "down":
		return ":rotating_light:"
	case "degraded":
		return ":warning:"
	case "ssl_expiring":
		return ":lock:"
	case "flapping":
		return ":cyclone:"
	case "stabilized":
		return ":large_blue_circle:"
	case "up":
		return ":white_check_mark:"
	default:
		return ":grey_question:"
	}
}

func eventLabel(eventType string) string {
	switch eventType {
	case "down":
		return "Down"
	case "degraded":
		return "Degraded"
	case "ssl_expiring":
		return "SSL Expiring"
	case "flapping":
		return "Flapping"
	case "stabilized":
		return "Stabilized"
	case "up":
		return "Up"
	default:
		return eventType
	}
}

// SendDigest dispatches a daily digest summary to all enabled notification channels.
// When events is empty an "all systems operational" message is sent so that operators
// receive a daily confirmation even on incident-free days.
func (s *Service) SendDigest(events []db.DigestEvent) {
	channels, err := s.store.GetNotificationChannels()
	if err != nil {
		log.Printf("Digest: failed to fetch channels: %v", err)
		return
	}

	// Group events by monitor (only when there are events to summarise).
	type monitorData struct {
		name       string
		url        string
		counts     map[string]int
		sslMessage string // latest SSL expiry message
	}
	byMonitor := make(map[string]*monitorData)
	var monitorOrder []string

	for _, e := range events {
		md, ok := byMonitor[e.MonitorID]
		if !ok {
			md = &monitorData{name: e.MonitorName, url: e.MonitorURL, counts: make(map[string]int)}
			byMonitor[e.MonitorID] = md
			monitorOrder = append(monitorOrder, e.MonitorID)
		}
		md.counts[e.EventType]++
		if e.EventType == "ssl_expiring" && e.Message != "" {
			md.sslMessage = e.Message
		}
	}

	// Build digestMonitor list.
	var monitors []digestMonitor
	for _, mid := range monitorOrder {
		md := byMonitor[mid]
		var types []string
		for t := range md.counts {
			types = append(types, t)
		}
		sort.Slice(types, func(i, j int) bool {
			return eventSeverity(types[i]) < eventSeverity(types[j])
		})

		var eventCounts []digestEventCount
		worstSeverity := 99
		for _, t := range types {
			eventCounts = append(eventCounts, digestEventCount{Type: t, Count: md.counts[t]})
			if sev := eventSeverity(t); sev < worstSeverity {
				worstSeverity = sev
			}
		}

		monitors = append(monitors, digestMonitor{
			Name:       md.name,
			URL:        md.url,
			Events:     eventCounts,
			Severity:   worstSeverity,
			SSLMessage: md.sslMessage,
		})
	}

	// Sort monitors by severity (most critical first).
	sort.SliceStable(monitors, func(i, j int) bool {
		return monitors[i].Severity < monitors[j].Severity
	})

	summary := digestSummary{
		TotalEvents:  len(events),
		MonitorCount: len(monitors),
		Monitors:     monitors,
		Date:         time.Now(),
	}

	for _, ch := range channels {
		if !ch.Enabled {
			continue
		}

		switch ch.Type {
		case "slack":
			n := NewSlackNotifier(ch.Config)
			if err := n.sendDigest(summary); err != nil {
				log.Printf("Digest: failed to send to Slack (%s): %v", ch.Name, err)
			}
		case "webhook":
			n := NewWebhookNotifier(ch.Config)
			if err := n.sendDigest(summary, events); err != nil {
				log.Printf("Digest: failed to send to webhook (%s): %v", ch.Name, err)
			}
		}
	}
}

func (n *SlackNotifier) sendDigest(summary digestSummary) error {
	webhookURL, ok := n.config["webhookUrl"].(string)
	if !ok || webhookURL == "" {
		return fmt.Errorf("webhookUrl missing or invalid")
	}

	dateStr := summary.Date.Format("January 2, 2006")

	var subtitle, fallbackText string
	if summary.TotalEvents == 0 {
		subtitle = fmt.Sprintf(":white_check_mark: %s  ·  All systems operational", dateStr)
		fallbackText = ":bar_chart: Daily Monitoring Summary — All systems operational"
	} else {
		subtitle = fmt.Sprintf(":clock3: %s  ·  %d events across %d monitors",
			dateStr, summary.TotalEvents, summary.MonitorCount)
		fallbackText = fmt.Sprintf(":bar_chart: Daily Monitoring Summary — %d events across %d monitors",
			summary.TotalEvents, summary.MonitorCount)
	}

	blocks := []map[string]interface{}{
		{
			"type": "header",
			"text": map[string]interface{}{
				"type":  "plain_text",
				"text":  ":bar_chart: Daily Monitoring Summary",
				"emoji": true,
			},
		},
		{
			"type": "context",
			"elements": []map[string]interface{}{
				{
					"type": "mrkdwn",
					"text": subtitle,
				},
			},
		},
		{
			"type": "divider",
		},
	}

	if summary.TotalEvents == 0 {
		blocks = append(blocks, map[string]interface{}{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": ":white_check_mark:  All systems operational — no incidents today.",
			},
		})
	} else {
		for _, m := range summary.Monitors {
			var eventParts []string
			for _, ec := range m.Events {
				if ec.Type == "ssl_expiring" && m.SSLMessage != "" {
					eventParts = append(eventParts, fmt.Sprintf("%s %s",
						eventEmoji(ec.Type), m.SSLMessage))
				} else {
					eventParts = append(eventParts, fmt.Sprintf("%s %s `%dx`",
						eventEmoji(ec.Type), eventLabel(ec.Type), ec.Count))
				}
			}

			monitorEmoji := ":white_check_mark:"
			if len(m.Events) > 0 {
				monitorEmoji = eventEmoji(m.Events[0].Type)
			}

			text := fmt.Sprintf("%s  *%s*\n%s",
				monitorEmoji, m.Name, strings.Join(eventParts, "  ·  "))

			blocks = append(blocks, map[string]interface{}{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": text,
				},
			})
		}
	}

	payload := map[string]interface{}{
		"text":   fallbackText,
		"blocks": blocks,
	}

	return sendJSON(webhookURL, payload)
}

func (n *WebhookNotifier) sendDigest(summary digestSummary, events []db.DigestEvent) error {
	webhookURL, ok := n.config["webhookUrl"].(string)
	if !ok || webhookURL == "" {
		return fmt.Errorf("webhookUrl missing or invalid")
	}

	// Initialise as empty slice (not nil) so the JSON field is always an array,
	// never null — even when there are no events (all-clear digest).
	monitorSummaries := make([]map[string]interface{}, 0, len(summary.Monitors))
	for _, m := range summary.Monitors {
		eventCounts := make(map[string]int)
		for _, ec := range m.Events {
			eventCounts[ec.Type] = ec.Count
		}
		monitorSummaries = append(monitorSummaries, map[string]interface{}{
			"name":   m.Name,
			"url":    m.URL,
			"events": eventCounts,
		})
	}

	// Build plain-text summary line.
	var summaryText string
	if summary.TotalEvents == 0 {
		summaryText = "All systems operational — no incidents today."
	} else {
		var lines []string
		for _, m := range summary.Monitors {
			var parts []string
			for _, ec := range m.Events {
				if ec.Type == "ssl_expiring" && m.SSLMessage != "" {
					parts = append(parts, m.SSLMessage)
				} else {
					parts = append(parts, fmt.Sprintf("%s (%dx)", ec.Type, ec.Count))
				}
			}
			lines = append(lines, fmt.Sprintf("- %s: %s", m.Name, strings.Join(parts, ", ")))
		}
		summaryText = strings.Join(lines, "\n")
	}

	var title string
	if summary.TotalEvents == 0 {
		title = "Daily Monitoring Summary (all systems operational)"
	} else {
		title = fmt.Sprintf("Daily Monitoring Summary (%d events)", summary.TotalEvents)
	}

	payload := map[string]interface{}{
		"type":         "digest",
		"title":        title,
		"summary":      summaryText,
		"eventCount":   summary.TotalEvents,
		"monitorCount": summary.MonitorCount,
		"monitors":     monitorSummaries,
		"timestamp":    summary.Date.Format(time.RFC3339),
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
