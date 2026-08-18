package uptime

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/notifications"
)

// How often the evaluator re-reads open outages and decides whether any of them has
// earned an alert. Well below the smallest useful sustained threshold, so the alert lands
// within a tick of the threshold rather than a tick of the check interval.
const alertEvalInterval = 15 * time.Second

// alertPolicy is the duration ladder an open outage climbs. Everything the evaluator
// decides comes from these three numbers plus the outage's own timestamps.
//
// A zero SustainedFor means "no silent window": alert as soon as the outage exists, which
// is the pre-3.x behaviour for anyone who wants it back. A zero FirstReminderAfter turns
// reminders off entirely.
type alertPolicy struct {
	SustainedFor        time.Duration
	FirstReminderAfter  time.Duration
	RepeatReminderAfter time.Duration
}

func defaultAlertPolicy() alertPolicy {
	return alertPolicy{
		SustainedFor:        3 * time.Minute,
		FirstReminderAfter:  30 * time.Minute,
		RepeatReminderAfter: time.Hour,
	}
}

type alertAction int

const (
	alertNone alertAction = iota
	alertFire
	alertRemind
)

// decideAlertAction is the whole policy, as a pure function of the outage, the clock and
// the ladder. Kept free of I/O so the interesting cases are unit tests rather than
// something you have to wait three minutes to observe.
func decideAlertAction(o db.OpenOutage, now time.Time, p alertPolicy) alertAction {
	if o.NotifiedAt == nil {
		if now.Sub(o.StartTime) >= p.SustainedFor {
			return alertFire
		}
		return alertNone
	}

	if p.FirstReminderAfter <= 0 {
		return alertNone
	}

	last := *o.NotifiedAt
	due := p.FirstReminderAfter
	if o.LastReminderAt != nil && o.LastReminderAt.After(last) {
		last = *o.LastReminderAt
		due = p.RepeatReminderAfter
		if due <= 0 {
			return alertNone
		}
	}

	if now.Sub(last) >= due {
		return alertRemind
	}
	return alertNone
}

// formatAlertDuration renders a span the way an operator reads it at a glance: "45s",
// "3m", "1h07m". Seconds disappear once minutes are on the clock — nobody acts on them.
func formatAlertDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	h := int(d.Hours())
	return fmt.Sprintf("%dh%02dm", h, int(d.Minutes())-h*60)
}

func (m *Manager) alertEvaluator() {
	m.wg.Add(1)
	defer m.wg.Done()

	ticker := time.NewTicker(alertEvalInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.evaluateAlerts(time.Now())
		}
	}
}

// evaluateAlerts walks every open outage and sends whatever the ladder says is due. It is
// the only place a down or degraded alert is emitted; the result processor's job is to
// open and close outages, not to decide who hears about them.
func (m *Manager) evaluateAlerts(now time.Time) {
	outages, err := m.store.GetOpenOutages()
	if err != nil {
		log.Printf("Alerting: failed to read open outages: %v", err)
		return
	}
	if len(outages) == 0 {
		return
	}

	m.mu.RLock()
	policy := m.alertPolicy
	filter := m.eventFilter
	m.mu.RUnlock()

	for _, o := range outages {
		mon := m.GetMonitor(o.MonitorID)
		if mon == nil {
			// Paused or not yet synced. Its outage stays open on purpose (see Sync), but a
			// monitor nobody is checking has nothing new to say.
			continue
		}
		if !filter.IsEnabled(o.Type) || m.isMonitorInMaintenance(mon.GetGroupID()) || mon.IsFlapping() {
			continue
		}

		switch decideAlertAction(o, now, policy) {
		case alertFire:
			claimed, err := m.store.MarkOutageNotified(o.ID, now)
			if err != nil {
				log.Printf("Alerting: failed to stamp outage %d as notified: %v", o.ID, err)
				continue
			}
			if !claimed {
				continue
			}
			m.notifyNow(m.outageEvent(o, now, false))
			log.Printf("Alerting: %s alert for %s after %s", o.Type, o.MonitorID,
				formatAlertDuration(now.Sub(o.StartTime)))
		case alertRemind:
			if err := m.store.MarkOutageReminded(o.ID, now); err != nil {
				log.Printf("Alerting: failed to stamp outage %d as reminded: %v", o.ID, err)
				continue
			}
			m.notifyNow(m.outageEvent(o, now, true))
		}
	}
}

func (m *Manager) outageEvent(o db.OpenOutage, now time.Time, reminder bool) notifications.NotificationEvent {
	kind := notifications.EventDown
	state := "down"
	if o.Type == "degraded" {
		kind = notifications.EventDegraded
		state = "degraded"
	}

	elapsed := formatAlertDuration(now.Sub(o.StartTime))
	message := fmt.Sprintf("%s — %s for %s", o.Summary, state, elapsed)
	if o.Summary == "" {
		// A summary can come back empty from an older row or a failed write. Leading with
		// the separator would make the alert read as though something had been cut off.
		message = fmt.Sprintf("Monitor is %s for %s", state, elapsed)
	}
	if reminder {
		message = fmt.Sprintf("Still %s after %s", state, elapsed)
	}

	return notifications.NotificationEvent{
		MonitorID:   o.MonitorID,
		MonitorName: o.MonitorName,
		MonitorURL:  o.MonitorURL,
		Type:        kind,
		Message:     message,
		Time:        now,
	}
}

// loadAlertPolicy reads the duration ladder from settings, falling back to the defaults
// for anything missing or nonsensical.
func (m *Manager) loadAlertPolicy() alertPolicy {
	p := defaultAlertPolicy()

	if v, ok := m.settingSeconds("notification.alert.sustained_seconds"); ok {
		p.SustainedFor = v
	}
	if v, ok := m.settingMinutes("notification.alert.reminder_minutes"); ok {
		p.FirstReminderAfter = v
	}
	if v, ok := m.settingMinutes("notification.alert.repeat_reminder_minutes"); ok {
		p.RepeatReminderAfter = v
	}

	return p
}

func (m *Manager) settingSeconds(key string) (time.Duration, bool) {
	return m.settingDuration(key, time.Second)
}

func (m *Manager) settingMinutes(key string) (time.Duration, bool) {
	return m.settingDuration(key, time.Minute)
}

// settingDuration reads a non-negative integer setting and scales it. Zero is a legal
// value with meaning (no silent window, no reminders), so it is accepted; negatives and
// unparseable values are not, and leave the default in place.
func (m *Manager) settingDuration(key string, unit time.Duration) (time.Duration, bool) {
	val, err := m.store.GetSetting(key)
	if err != nil || val == "" {
		return 0, false
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 0 {
		return 0, false
	}
	return time.Duration(n) * unit, true
}
