package uptime

import (
	"strings"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

func at(base time.Time, d time.Duration) *time.Time {
	t := base.Add(d)
	return &t
}

func TestDecideAlertAction_StaysSilentUntilSustained(t *testing.T) {
	p := defaultAlertPolicy()
	start := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	o := db.OpenOutage{StartTime: start, Type: "down"}

	cases := []struct {
		age  time.Duration
		want alertAction
	}{
		{0, alertNone},
		{59 * time.Second, alertNone},
		{2*time.Minute + 59*time.Second, alertNone},
		{3 * time.Minute, alertFire},
		{10 * time.Minute, alertFire},
	}

	for _, c := range cases {
		if got := decideAlertAction(o, start.Add(c.age), p); got != c.want {
			t.Errorf("after %s: got %v, want %v", c.age, got, c.want)
		}
	}
}

func TestDecideAlertAction_ReminderLadder(t *testing.T) {
	p := defaultAlertPolicy()
	start := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)

	// Announced at the 3-minute mark, no reminder sent yet: the first reminder is due 30
	// minutes after the announcement, not 30 minutes after the outage began.
	notified := db.OpenOutage{StartTime: start, Type: "down", NotifiedAt: at(start, 3*time.Minute)}

	if got := decideAlertAction(notified, start.Add(32*time.Minute), p); got != alertNone {
		t.Errorf("29m after the alert: got %v, want alertNone", got)
	}
	if got := decideAlertAction(notified, start.Add(33*time.Minute), p); got != alertRemind {
		t.Errorf("30m after the alert: got %v, want alertRemind", got)
	}

	// Once a reminder has gone out the interval widens to the repeat cadence.
	reminded := notified
	reminded.LastReminderAt = at(start, 33*time.Minute)

	if got := decideAlertAction(reminded, start.Add(92*time.Minute), p); got != alertNone {
		t.Errorf("59m after the reminder: got %v, want alertNone", got)
	}
	if got := decideAlertAction(reminded, start.Add(93*time.Minute), p); got != alertRemind {
		t.Errorf("60m after the reminder: got %v, want alertRemind", got)
	}
}

func TestDecideAlertAction_ZeroSustainedAlertsImmediately(t *testing.T) {
	p := defaultAlertPolicy()
	p.SustainedFor = 0
	start := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	o := db.OpenOutage{StartTime: start, Type: "down"}

	if got := decideAlertAction(o, start, p); got != alertFire {
		t.Errorf("with no silent window: got %v, want alertFire", got)
	}
}

func TestDecideAlertAction_ZeroReminderDisablesReminders(t *testing.T) {
	start := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	o := db.OpenOutage{StartTime: start, Type: "down", NotifiedAt: at(start, 3*time.Minute)}

	p := defaultAlertPolicy()
	p.FirstReminderAfter = 0
	if got := decideAlertAction(o, start.Add(24*time.Hour), p); got != alertNone {
		t.Errorf("with reminders off: got %v, want alertNone", got)
	}

	// A first reminder still fires, but the repeat cadence being off stops the next one.
	p = defaultAlertPolicy()
	p.RepeatReminderAfter = 0
	o.LastReminderAt = at(start, 33*time.Minute)
	if got := decideAlertAction(o, start.Add(24*time.Hour), p); got != alertNone {
		t.Errorf("with repeats off: got %v, want alertNone", got)
	}
}

func TestFormatAlertDuration(t *testing.T) {
	cases := map[time.Duration]string{
		45 * time.Second:             "45s",
		3 * time.Minute:              "3m",
		59 * time.Minute:             "59m",
		time.Hour + 7*time.Minute:    "1h07m",
		3*time.Hour + 22*time.Minute: "3h22m",
		25*time.Hour + 5*time.Minute: "25h05m",
	}
	for d, want := range cases {
		if got := formatAlertDuration(d); got != want {
			t.Errorf("formatAlertDuration(%s) = %q, want %q", d, got, want)
		}
	}
}

func TestOutageEvent_MessageShape(t *testing.T) {
	m := &Manager{}
	start := time.Date(2026, 8, 17, 0, 45, 0, 0, time.UTC)
	o := db.OpenOutage{
		MonitorID:   "m-jellyfin",
		MonitorName: "Jellyfin",
		MonitorURL:  "https://jellyfin.example.com",
		Type:        "down",
		Summary:     "Monitor is down (Status: 503)",
		StartTime:   start,
	}

	ev := m.outageEvent(o, start.Add(3*time.Minute), false)
	if want := "Monitor is down (Status: 503) — down for 3m"; ev.Message != want {
		t.Errorf("first alert message = %q, want %q", ev.Message, want)
	}
	if ev.MonitorName != "Jellyfin" || ev.MonitorURL != "https://jellyfin.example.com" {
		t.Errorf("event lost monitor identity: %+v", ev)
	}

	rem := m.outageEvent(o, start.Add(time.Hour+7*time.Minute), true)
	if want := "Still down after 1h07m"; rem.Message != want {
		t.Errorf("reminder message = %q, want %q", rem.Message, want)
	}

	o.Type = "degraded"
	deg := m.outageEvent(o, start.Add(5*time.Minute), true)
	if want := "Still degraded after 5m"; deg.Message != want {
		t.Errorf("degraded reminder = %q, want %q", deg.Message, want)
	}
}

// --- edge cases ---

// Clock skew, or a start time written by a database in another timezone, makes the elapsed
// span negative. The ladder must read that as "not yet", never as "overdue": alerting on a
// negative age would fire on every outage the instant it opened.
func TestDecideAlertAction_NegativeElapsedStaysSilent(t *testing.T) {
	p := defaultAlertPolicy()
	start := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	o := db.OpenOutage{StartTime: start, Type: "down"}

	if got := decideAlertAction(o, start.Add(-2*time.Hour), p); got != alertNone {
		t.Errorf("an outage that starts in the future: got %v, want alertNone", got)
	}
}

// Exactly on the boundary counts as met. An off-by-one here delays every alert by a full
// evaluator tick, which is invisible in production and infuriating to debug.
func TestDecideAlertAction_BoundaryIsInclusive(t *testing.T) {
	p := defaultAlertPolicy()
	start := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

	if got := decideAlertAction(db.OpenOutage{StartTime: start}, start.Add(p.SustainedFor), p); got != alertFire {
		t.Errorf("exactly at the sustained window: got %v, want alertFire", got)
	}

	notified := db.OpenOutage{StartTime: start, NotifiedAt: at(start, p.SustainedFor)}
	due := start.Add(p.SustainedFor).Add(p.FirstReminderAfter)
	if got := decideAlertAction(notified, due, p); got != alertRemind {
		t.Errorf("exactly at the first reminder: got %v, want alertRemind", got)
	}
	if got := decideAlertAction(notified, due.Add(-time.Nanosecond), p); got != alertNone {
		t.Errorf("a nanosecond before the reminder: got %v, want alertNone", got)
	}
}

// A reminder stamped before the announcement — possible if the clock moved backwards, or a
// row was edited by hand — must not make the ladder compute a reminder from the older mark.
func TestDecideAlertAction_IgnoresAStaleReminderStamp(t *testing.T) {
	p := defaultAlertPolicy()
	start := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	o := db.OpenOutage{
		StartTime:      start,
		NotifiedAt:     at(start, time.Hour),
		LastReminderAt: at(start, 5*time.Minute), // before the announcement
	}

	// The announcement is the later mark, so the first reminder is due 30m after it.
	if got := decideAlertAction(o, start.Add(time.Hour).Add(29*time.Minute), p); got != alertNone {
		t.Errorf("29m after the announcement: got %v, want alertNone", got)
	}
	if got := decideAlertAction(o, start.Add(time.Hour).Add(30*time.Minute), p); got != alertRemind {
		t.Errorf("30m after the announcement: got %v, want alertRemind", got)
	}
}

// Every span an operator can actually see, including the ones at the unit boundaries.
func TestFormatAlertDuration_Boundaries(t *testing.T) {
	cases := map[time.Duration]string{
		0:                                     "0s",
		999 * time.Millisecond:                "0s",
		59*time.Second + 999*time.Millisecond: "59s",
		time.Minute:                           "1m",
		time.Hour - time.Second:               "59m",
		time.Hour:                             "1h00m",
		24 * time.Hour:                        "24h00m",
	}
	for d, want := range cases {
		if got := formatAlertDuration(d); got != want {
			t.Errorf("formatAlertDuration(%s) = %q, want %q", d, got, want)
		}
	}
}

// A summary that came back empty from the database must not produce a message that opens
// with a dangling separator.
func TestOutageEvent_HandlesAnEmptySummary(t *testing.T) {
	m := &Manager{}
	start := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	o := db.OpenOutage{MonitorID: "m1", MonitorName: "API", Type: "down", StartTime: start}

	ev := m.outageEvent(o, start.Add(3*time.Minute), false)
	if ev.Message == "" {
		t.Fatal("empty message")
	}
	if strings.HasPrefix(ev.Message, "—") || strings.HasPrefix(ev.Message, " ") {
		t.Errorf("message starts with a dangling separator: %q", ev.Message)
	}
	if !strings.Contains(ev.Message, "3m") {
		t.Errorf("message lost the duration: %q", ev.Message)
	}
}
