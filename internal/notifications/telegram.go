package notifications

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

const telegramMessageLimit = 4096

type TelegramNotifier struct {
	config map[string]interface{}
	apiURL string
}

func NewTelegramNotifier(configJSON string) *TelegramNotifier {
	var config map[string]interface{}
	_ = json.Unmarshal([]byte(configJSON), &config)
	return &TelegramNotifier{config: config, apiURL: "https://api.telegram.org"}
}

func (n *TelegramNotifier) Send(event NotificationEvent) error {
	return n.sendText(fmt.Sprintf("%s: %s\n%s\n%s\n%s",
		eventTitle(event.Type), event.MonitorName, event.Message, event.MonitorURL,
		event.Time.Format("2006-01-02 15:04:05 MST")))
}

func (n *TelegramNotifier) sendDigest(summary digestSummary) error {
	var body strings.Builder
	body.WriteString("Daily Monitoring Summary\n")
	if summary.TotalEvents == 0 {
		body.WriteString("All systems operational — no incidents today.")
	} else {
		body.WriteString(fmt.Sprintf("%d events across %d monitors", summary.TotalEvents, summary.MonitorCount))
		for _, monitor := range summary.Monitors {
			body.WriteString("\n• " + monitor.Name)
			for _, event := range monitor.Events {
				body.WriteString(fmt.Sprintf(" — %s (%dx)", eventLabel(event.Type), event.Count))
			}
		}
	}
	return n.sendText(body.String())
}

func (n *TelegramNotifier) sendText(text string) error {
	botToken, _ := n.config["botToken"].(string)
	chatID, _ := n.config["chatId"].(string)
	if botToken == "" || chatID == "" {
		return fmt.Errorf("botToken and chatId are required")
	}
	for _, chunk := range telegramChunks(text) {
		payload := map[string]interface{}{"chat_id": chatID, "text": chunk, "disable_web_page_preview": true}
		if err := sendJSON(n.apiURL+"/bot"+botToken+"/sendMessage", payload); err != nil {
			return fmt.Errorf("telegram API request failed: %s", strings.ReplaceAll(err.Error(), botToken, "[redacted]"))
		}
	}
	return nil
}

func telegramChunks(text string) []string {
	if utf8.RuneCountInString(text) <= telegramMessageLimit {
		return []string{text}
	}
	runes := []rune(text)
	chunks := make([]string, 0, (len(runes)+telegramMessageLimit-1)/telegramMessageLimit)
	for len(runes) > 0 {
		end := min(telegramMessageLimit, len(runes))
		chunks = append(chunks, string(runes[:end]))
		runes = runes[end:]
	}
	return chunks
}
