package notifications

import "time"

func slackAlertPayload(event NotificationEvent) map[string]interface{} {
	title := eventTitle(event.Type) + ": " + event.MonitorName
	if event.Type == EventBatch {
		title = event.MonitorName
	}
	message := event.Message
	if message == "" {
		message = eventTitle(event.Type)
	}
	blocks := []map[string]interface{}{
		{"type": "header", "text": map[string]interface{}{"type": "plain_text", "text": clipNotification(title, 150)}},
		{"type": "section", "text": map[string]interface{}{"type": "plain_text", "text": clipNotification(message, 3000)}},
		{"type": "context", "elements": []map[string]interface{}{{"type": "plain_text", "text": "Warden · " + event.Time.UTC().Format(time.RFC3339)}}},
	}
	return map[string]interface{}{
		"text":        slackEscapeLinkText(clipNotification(title+" — "+message, 1800)),
		"attachments": []map[string]interface{}{{"color": eventColor(notificationSeverity(event)), "blocks": blocks}},
	}
}
