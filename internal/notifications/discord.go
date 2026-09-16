package notifications

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	discordTitleLimit       = 256
	discordDescriptionLimit = 4096
	discordFieldLimit       = 1024
)

type DiscordNotifier struct {
	config map[string]interface{}
}

func NewDiscordNotifier(configJSON string) *DiscordNotifier {
	var config map[string]interface{}
	_ = json.Unmarshal([]byte(configJSON), &config)
	return &DiscordNotifier{config: config}
}

func (n *DiscordNotifier) Send(event NotificationEvent) error {
	webhookURL, _ := n.config["webhookUrl"].(string)
	if webhookURL == "" {
		return fmt.Errorf("webhookUrl missing or invalid")
	}
	payload := map[string]interface{}{
		"username":         "Warden",
		"allowed_mentions": map[string]interface{}{"parse": []string{}},
		"embeds": []map[string]interface{}{{
			"title":       truncateDiscord(eventTitle(event.Type)+": "+event.MonitorName, discordTitleLimit),
			"description": truncateDiscord(event.Message, discordDescriptionLimit),
			"color":       discordColor(event.Type),
			"fields":      []map[string]interface{}{{"name": "Monitor", "value": truncateDiscord(event.MonitorURL, discordFieldLimit), "inline": false}},
			"timestamp":   event.Time.Format(time.RFC3339),
		}},
	}
	if err := sendJSON(webhookURL, payload); err != nil {
		return fmt.Errorf("discord webhook request failed: %s", strings.ReplaceAll(err.Error(), webhookURL, "[redacted]"))
	}
	return nil
}

func (n *DiscordNotifier) sendDigest(summary digestSummary) error {
	webhookURL, _ := n.config["webhookUrl"].(string)
	if webhookURL == "" {
		return fmt.Errorf("webhookUrl missing or invalid")
	}
	description := "All systems operational — no incidents today."
	if summary.TotalEvents > 0 {
		lines := make([]string, 0, len(summary.Monitors))
		for _, monitor := range summary.Monitors {
			parts := make([]string, 0, len(monitor.Events))
			for _, event := range monitor.Events {
				parts = append(parts, fmt.Sprintf("%s (%dx)", eventLabel(event.Type), event.Count))
			}
			lines = append(lines, fmt.Sprintf("**%s** — %s", monitor.Name, strings.Join(parts, ", ")))
		}
		description = truncateDiscord(strings.Join(lines, "\n"), discordDescriptionLimit)
	}
	payload := map[string]interface{}{
		"username":         "Warden",
		"allowed_mentions": map[string]interface{}{"parse": []string{}},
		"embeds": []map[string]interface{}{{
			"title":       "Daily Monitoring Summary",
			"description": description,
			"color":       0x3b82f6,
			"timestamp":   summary.Date.Format(time.RFC3339),
		}},
	}
	if err := sendJSON(webhookURL, payload); err != nil {
		return fmt.Errorf("discord webhook request failed: %s", strings.ReplaceAll(err.Error(), webhookURL, "[redacted]"))
	}
	return nil
}

func truncateDiscord(value string, limit int) string {
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit-1]) + "…"
}

func discordColor(eventType EventType) int {
	switch eventType {
	case EventDown:
		return 0xef4444
	case EventDegraded, EventSSLExpiring:
		return 0xf59e0b
	case EventUp, EventStabilized:
		return 0x22c55e
	default:
		return 0x3b82f6
	}
}
