package uptime

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/notifications"
)

// weeklyInsightsSentKey remembers the last week already reported, in the database rather
// than in memory. The daily digest keeps its marker in a local variable and consequently
// forgets across restarts; there is no reason to repeat that here.
const weeklyInsightsSentKey = "notification.insights.last_sent_week"

// weeklyInsightsConfig is when, if ever, the pattern summary goes out. Off by default:
// this is a new kind of message, and an install that upgrades should not suddenly start
// receiving one.
type weeklyInsightsConfig struct {
	Enabled bool
	Weekday time.Weekday
	AtTime  string // HH:MM in the notification timezone
}

func defaultWeeklyInsightsConfig() weeklyInsightsConfig {
	return weeklyInsightsConfig{Enabled: false, Weekday: time.Monday, AtTime: "09:00"}
}

func (m *Manager) loadWeeklyInsightsConfig() weeklyInsightsConfig {
	c := defaultWeeklyInsightsConfig()

	if v, err := m.store.GetSetting("notification.insights.weekly_enabled"); err == nil && v != "" {
		c.Enabled = v == "true"
	}
	if v, ok := m.settingInt("notification.insights.weekly_day"); ok && v <= 6 {
		c.Weekday = time.Weekday(v)
	}
	if v, err := m.store.GetSetting("notification.insights.weekly_time"); err == nil && v != "" {
		c.AtTime = v
	}

	return c
}

func (m *Manager) weeklyInsightsWorker() {
	m.wg.Add(1)
	defer m.wg.Done()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.maybeSendWeeklyInsights(time.Now())
		}
	}
}

// maybeSendWeeklyInsights sends at most one summary per ISO week. Split out from the loop
// so a test can drive the clock instead of waiting a week.
func (m *Manager) maybeSendWeeklyInsights(now time.Time) {
	m.mu.RLock()
	cfg := m.weeklyInsights
	loc := m.notificationTimezone
	m.mu.RUnlock()

	if !cfg.Enabled {
		return
	}

	local := now.In(loc)
	if local.Weekday() != cfg.Weekday || local.Format("15:04") < cfg.AtTime {
		return
	}

	year, week := local.ISOWeek()
	stamp := fmt.Sprintf("%d-W%02d", year, week)

	if last, err := m.store.GetSetting(weeklyInsightsSentKey); err == nil && last == stamp {
		return
	}

	findings, err := m.store.GetMonitorInsights("")
	if err != nil {
		// Leave the marker alone so the next tick retries rather than skipping the week.
		log.Printf("Insights: failed to load findings for the weekly summary: %v", err)
		return
	}
	if len(findings) == 0 {
		// Nothing found is not worth a message. The daily digest already confirms that
		// Warden is alive; a weekly "no patterns" would just be another thing to ignore.
		if err := m.store.SetSetting(weeklyInsightsSentKey, stamp); err != nil {
			log.Printf("Insights: failed to record the weekly marker: %v", err)
		}
		return
	}

	m.notifyNow(weeklyInsightsEvent(findings, local))

	if err := m.store.SetSetting(weeklyInsightsSentKey, stamp); err != nil {
		log.Printf("Insights: failed to record the weekly marker: %v", err)
	}
	log.Printf("Insights: sent the weekly summary with %d findings for %s", len(findings), stamp)
}

// weeklyInsightsEvent renders the findings as one message, grouped by monitor and ordered
// with the strongest first. Confident findings go at the top because a reader who stops
// after two lines should have read the two that matter.
func weeklyInsightsEvent(findings []db.MonitorInsight, at time.Time) notifications.NotificationEvent {
	sorted := append([]db.MonitorInsight(nil), findings...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Confidence != sorted[j].Confidence {
			return sorted[i].Confidence == "high"
		}
		return sorted[i].MonitorName < sorted[j].MonitorName
	})

	var lines []string
	for _, f := range sorted {
		lines = append(lines, "• "+f.Summary)
	}

	monitors := map[string]struct{}{}
	for _, f := range findings {
		monitors[f.MonitorID] = struct{}{}
	}

	header := fmt.Sprintf("%d pattern(s) across %d monitor(s), from the last 14 days",
		len(findings), len(monitors))

	return notifications.NotificationEvent{
		MonitorName: "Weekly patterns",
		Type:        notifications.EventStabilized,
		Message:     header + "\n" + strings.Join(lines, "\n"),
		Time:        at,
	}
}
