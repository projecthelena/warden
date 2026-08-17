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

// evaluateAlerts is the whole alerting decision, once per tick. The order matters, and it
// runs widest-first: if everything is broken that is one message, if a group is broken
// that is one message, and only what is left over is judged monitor by monitor.
//
// It is the only place a down or degraded alert is emitted; the result processor's job is
// to open and close outages, not to decide who hears about them.
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
	corr := m.correlationPolicy
	filter := m.eventFilter
	m.mu.RUnlock()

	eligible, groupSizes, totalMonitors := m.partitionOutages(outages, filter)

	var pending []db.OpenOutage
	for _, o := range eligible {
		if o.NotifiedAt == nil {
			pending = append(pending, o)
		}
	}

	// 1. Everything is down. Warden watches from one place, so this is far more likely to
	//    be that place than every unrelated service failing at once.
	if rest, handled := m.announceProbeFailure(pending, now, policy, corr, totalMonitors); handled {
		pending = rest
	}

	// 2. A group failing together is one incident with a shared cause.
	pending = m.announceGroupFailures(pending, now, policy, corr, groupSizes)

	// 3. Whatever is left stands on its own.
	for _, o := range pending {
		m.announceSingle(o, now, policy, corr)
	}

	// 4. Reminders for what is already known to be broken, one per incident.
	m.sendReminders(eligible, now, policy)
}

// partitionOutages drops everything the evaluator must stay quiet about and reports the
// monitor counts the correlation thresholds are measured against.
func (m *Manager) partitionOutages(outages []db.OpenOutage, filter NotificationEventFilter) (eligible []db.OpenOutage, groupSizes map[string]int, total int) {
	groupSizes = make(map[string]int)
	for _, mon := range m.GetAll() {
		groupSizes[mon.GetGroupID()]++
		total++
	}

	for _, o := range outages {
		mon := m.GetMonitor(o.MonitorID)
		if mon == nil {
			// Paused or not yet synced. Its outage stays open on purpose (see Sync), but a
			// monitor nobody is checking has nothing new to say.
			continue
		}
		if o.AlertsMuted {
			continue
		}
		if !filter.IsEnabled(o.Type) || m.isMonitorInMaintenance(mon.GetGroupID()) || mon.IsFlapping() {
			continue
		}
		eligible = append(eligible, o)
	}
	return eligible, groupSizes, total
}

// announceProbeFailure catches the case where Warden itself is the problem. Reporting
// "Google, GitHub and Cloudflare are all down" as eighteen separate incidents sends you to
// debug eighteen innocent services.
//
// The failures must span at least two groups. A single group going down entirely looks
// identical from here, and "your NodeSource group is down" is both more specific and safer
// to be wrong about than "your network is broken".
func (m *Manager) announceProbeFailure(pending []db.OpenOutage, now time.Time, policy alertPolicy, corr correlationPolicy, totalMonitors int) ([]db.OpenOutage, bool) {
	if totalMonitors < corr.MinMonitors {
		return pending, false
	}

	var down []db.OpenOutage
	for _, o := range pending {
		if o.Type == "down" {
			down = append(down, o)
		}
	}
	if len(down) == 0 {
		return pending, false
	}

	needed := (totalMonitors*corr.ProbePercent + 99) / 100
	affected := distinctMonitors(down)
	if affected < needed || affected < corr.MinMonitors {
		return pending, false
	}
	if distinctGroups(down) < 2 {
		return pending, false
	}
	if decideAlertAction(down[0], now, policy) != alertFire {
		return pending, false
	}

	key := correlationKey("probe", down[0])
	claimed, err := m.store.MarkOutagesNotified(outageIDs(down), now, key)
	if err != nil {
		log.Printf("Alerting: failed to stamp probe-wide outage: %v", err)
		return pending, false
	}
	if claimed == 0 {
		return pending, false
	}

	m.notifyNow(notifications.NotificationEvent{
		MonitorID:   "",
		MonitorName: fmt.Sprintf("%d of %d monitors", affected, totalMonitors),
		Type:        notifications.EventDown,
		Message: fmt.Sprintf(
			"%d of %d monitors are down across every group, for %s. That is more likely to be Warden's own network than every target failing at once — check this host before the services. Affected: %s",
			affected, totalMonitors, formatAlertDuration(now.Sub(down[0].StartTime)), monitorList(down, 8)),
		Time: now,
	})
	log.Printf("Alerting: probe-wide failure, %d of %d monitors down", affected, totalMonitors)

	return nil, true
}

// announceGroupFailures turns "eleven monitors in NodeSource returned 404 within two
// minutes" into one message, and returns whatever was not part of a group incident.
func (m *Manager) announceGroupFailures(pending []db.OpenOutage, now time.Time, policy alertPolicy, corr correlationPolicy, groupSizes map[string]int) []db.OpenOutage {
	byGroup := make(map[string][]db.OpenOutage)
	for _, o := range pending {
		byGroup[o.GroupID] = append(byGroup[o.GroupID], o)
	}

	var leftover []db.OpenOutage
	for groupID, groupOutages := range byGroup {
		needed := corr.requiredForGroup(groupSizes[groupID])

		for _, c := range cluster(groupOutages, corr.Window) {
			affected := distinctMonitors(c)
			if affected < needed {
				leftover = append(leftover, c...)
				continue
			}
			// The cluster is anchored on its earliest member, so the group has been in
			// trouble at least that long.
			if decideAlertAction(c[0], now, policy) != alertFire {
				leftover = append(leftover, c...)
				continue
			}

			key := correlationKey("group", c[0])
			claimed, err := m.store.MarkOutagesNotified(outageIDs(c), now, key)
			if err != nil {
				log.Printf("Alerting: failed to stamp correlated outage: %v", err)
				leftover = append(leftover, c...)
				continue
			}
			if claimed == 0 {
				continue
			}

			m.notifyNow(notifications.NotificationEvent{
				MonitorID:   "",
				MonitorName: c[0].GroupName,
				Type:        notifications.EventDown,
				Message: fmt.Sprintf("%d of %d monitors in %s are down, for %s: %s",
					affected, groupSizes[groupID], c[0].GroupName,
					formatAlertDuration(now.Sub(c[0].StartTime)), monitorList(c, 8)),
				Time: now,
			})
			log.Printf("Alerting: correlated failure in group %s, %d monitors", groupID, affected)
		}
	}
	return leftover
}

// announceSingle handles one outage on its own, applying the repeat-offender damping. A
// monitor that has already interrupted you ChronicLimit times today is telling you about
// itself rather than about an event, so it says that once and then stops.
func (m *Manager) announceSingle(o db.OpenOutage, now time.Time, policy alertPolicy, corr correlationPolicy) {
	if decideAlertAction(o, now, policy) != alertFire {
		return
	}

	announced, err := m.store.CountAnnouncedOutagesSince(o.MonitorID, now.Add(-corr.ChronicWindow))
	if err != nil {
		log.Printf("Alerting: failed to count recent alerts for %s: %v", o.MonitorID, err)
		announced = 0
	}

	// Already over the limit: stay silent and leave the outage un-stamped, so it produces
	// no reminder and no recovery message either. It is still in the history and the
	// digest — it just stops interrupting.
	if corr.ChronicLimit > 0 && announced >= corr.ChronicLimit {
		return
	}

	claimed, err := m.store.MarkOutageNotified(o.ID, now)
	if err != nil {
		log.Printf("Alerting: failed to stamp outage %d as notified: %v", o.ID, err)
		return
	}
	if !claimed {
		return
	}

	// This one crosses the line. Say so plainly instead of pretending it is news, so the
	// silence that follows is something the operator chose to read about, not a mystery.
	if corr.ChronicLimit > 0 && announced == corr.ChronicLimit-1 {
		m.notifyNow(notifications.NotificationEvent{
			MonitorID:   o.MonitorID,
			MonitorName: o.MonitorName,
			MonitorURL:  o.MonitorURL,
			Type:        notifications.EventFlapping,
			Message: fmt.Sprintf(
				"%s has alerted %d times in the last %s and is down again. Muting its individual alerts until it settles — it stays in the daily digest and on the dashboard.",
				o.MonitorName, corr.ChronicLimit, formatAlertDuration(corr.ChronicWindow)),
			Time: now,
		})
		log.Printf("Alerting: %s is chronically unstable, damping its alerts", o.MonitorID)
		return
	}

	m.notifyNow(m.outageEvent(o, now, false))
	log.Printf("Alerting: %s alert for %s after %s", o.Type, o.MonitorID,
		formatAlertDuration(now.Sub(o.StartTime)))
}

// sendReminders nudges about what is still broken. Outages announced together share a
// correlation id and are reminded about together — eleven monitors that failed as one
// event should not turn into eleven reminders half an hour later.
func (m *Manager) sendReminders(eligible []db.OpenOutage, now time.Time, policy alertPolicy) {
	byIncident := make(map[string][]db.OpenOutage)
	var order []string
	for _, o := range eligible {
		if o.NotifiedAt == nil {
			continue
		}
		key := o.CorrelationID
		if key == "" {
			key = fmt.Sprintf("single-%d", o.ID)
		}
		if _, seen := byIncident[key]; !seen {
			order = append(order, key)
		}
		byIncident[key] = append(byIncident[key], o)
	}

	for _, key := range order {
		members := byIncident[key]
		if decideAlertAction(members[0], now, policy) != alertRemind {
			continue
		}

		ids := outageIDs(members)
		for _, id := range ids {
			if err := m.store.MarkOutageReminded(id, now); err != nil {
				log.Printf("Alerting: failed to stamp outage %d as reminded: %v", id, err)
				return
			}
		}

		if len(members) == 1 {
			m.notifyNow(m.outageEvent(members[0], now, true))
			continue
		}
		m.notifyNow(notifications.NotificationEvent{
			MonitorID:   "",
			MonitorName: members[0].GroupName,
			Type:        notifications.EventDown,
			Message: fmt.Sprintf("Still down after %s: %s",
				formatAlertDuration(now.Sub(members[0].StartTime)), monitorList(members, 8)),
			Time: now,
		})
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

// loadCorrelationPolicy reads the grouping thresholds from settings.
func (m *Manager) loadCorrelationPolicy() correlationPolicy {
	p := defaultCorrelationPolicy()

	if v, ok := m.settingSeconds("notification.correlation.window_seconds"); ok {
		p.Window = v
	}
	if v, ok := m.settingInt("notification.correlation.min_monitors"); ok && v >= 1 {
		p.MinMonitors = v
	}
	if v, ok := m.settingInt("notification.correlation.group_percent"); ok && v >= 1 && v <= 100 {
		p.GroupPercent = v
	}
	if v, ok := m.settingInt("notification.correlation.probe_percent"); ok && v >= 1 && v <= 100 {
		p.ProbePercent = v
	}
	if v, ok := m.settingInt("notification.chronic.limit"); ok {
		p.ChronicLimit = v
	}
	if v, ok := m.settingMinutes("notification.chronic.window_minutes"); ok && v > 0 {
		p.ChronicWindow = v
	}

	return p
}

func (m *Manager) settingInt(key string) (int, bool) {
	val, err := m.store.GetSetting(key)
	if err != nil || val == "" {
		return 0, false
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
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
