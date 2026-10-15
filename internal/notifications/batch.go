package notifications

import (
	"fmt"
	"sort"
	"strings"
)

// Group delivery without guessing that simultaneous symptoms have a common cause.
// Keep the constituent events for machine consumers, including state changes on
// the same monitor. Human messages show a bounded summary.
func batchEvents(events []NotificationEvent) NotificationEvent {
	if len(events) == 1 {
		return events[0]
	}
	ordered := append([]NotificationEvent(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return eventSeverity(string(ordered[i].Type)) < eventSeverity(string(ordered[j].Type))
	})
	var lines []string
	for i, event := range ordered {
		if i == 8 {
			break
		}
		name := strings.Join(strings.Fields(event.MonitorName), " ")
		message := strings.Join(strings.Fields(event.Message), " ")
		lines = append(lines, fmt.Sprintf("%s · %s — %s", clipNotification(name, 80), eventLabel(string(event.Type)), clipNotification(message, 180)))
	}
	if len(events) > 8 {
		lines = append(lines, fmt.Sprintf("And %d more updates. See the dashboard for details.", len(events)-8))
	}
	return NotificationEvent{
		Type:        EventBatch,
		MonitorName: fmt.Sprintf("%d monitoring updates", len(events)),
		Message:     strings.Join(lines, "\n"),
		Time:        events[len(events)-1].Time,
		Events:      append([]NotificationEvent(nil), events...),
	}
}

func clipNotification(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit-1]) + "…"
}

func notificationSeverity(event NotificationEvent) EventType {
	if len(event.Events) == 0 {
		return event.Type
	}
	worst := event.Events[0].Type
	for _, child := range event.Events[1:] {
		if eventSeverity(string(child.Type)) < eventSeverity(string(worst)) {
			worst = child.Type
		}
	}
	return worst
}
