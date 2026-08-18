package uptime

import (
	"strings"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/notifications"
)

func seedFinding(t *testing.T, store *db.Store, monitorID, kind, summary, confidence string) {
	t.Helper()
	existing, err := store.GetMonitorInsights(monitorID)
	if err != nil {
		t.Fatalf("GetMonitorInsights: %v", err)
	}
	existing = append(existing, db.MonitorInsight{
		Kind: kind, Summary: summary, Confidence: confidence,
	})
	if err := store.ReplaceMonitorInsights(monitorID, existing, time.Now().UTC()); err != nil {
		t.Fatalf("ReplaceMonitorInsights: %v", err)
	}
}

func weeklyTestManager(t *testing.T) (*Manager, *db.Store, *spyNotifier) {
	t.Helper()
	m, store := newAlertTestManager(t)
	spy := &spyNotifier{}
	m.notifier = spy
	m.weeklyInsights = weeklyInsightsConfig{Enabled: true, Weekday: time.Monday, AtTime: "09:00"}
	return m, store, spy
}

// 2026-08-17 is a Monday.
func mondayAt(hour, minute int) time.Time {
	return time.Date(2026, 8, 17, hour, minute, 0, 0, time.UTC)
}

func TestWeeklyInsights_SendsOncePerWeek(t *testing.T) {
	m, store, spy := weeklyTestManager(t)
	seedFinding(t, store, "m1", "latency_sawtooth", "m1 climbs and resets", "high")

	m.maybeSendWeeklyInsights(mondayAt(9, 0))
	if got := len(spy.events); got != 1 {
		t.Fatalf("expected 1 weekly summary, got %d", got)
	}

	// Every subsequent tick that day must stay quiet.
	m.maybeSendWeeklyInsights(mondayAt(9, 1))
	m.maybeSendWeeklyInsights(mondayAt(17, 30))
	if got := len(spy.events); got != 1 {
		t.Errorf("the weekly summary went out %d times", got)
	}

	// Next Monday is a new week and a new message.
	m.maybeSendWeeklyInsights(mondayAt(9, 0).AddDate(0, 0, 7))
	if got := len(spy.events); got != 2 {
		t.Errorf("expected a second summary the following week, got %d total", got)
	}
}

// The marker lives in the database, so a restart neither re-sends nor skips. The daily
// digest keeps its equivalent in memory and does exactly that.
func TestWeeklyInsights_MarkerSurvivesRestart(t *testing.T) {
	m, store, spy := weeklyTestManager(t)
	seedFinding(t, store, "m1", "latency_sawtooth", "m1 climbs and resets", "high")

	m.maybeSendWeeklyInsights(mondayAt(9, 0))
	if len(spy.events) != 1 {
		t.Fatalf("no summary sent")
	}

	// A "restarted" manager over the same store must not send again.
	restarted := NewManager(store)
	restartedSpy := &spyNotifier{}
	restarted.notifier = restartedSpy
	restarted.weeklyInsights = m.weeklyInsights
	restarted.maybeSendWeeklyInsights(mondayAt(9, 30))

	if len(restartedSpy.events) != 0 {
		t.Errorf("a restart re-sent the weekly summary: %+v", restartedSpy.events)
	}
}

func TestWeeklyInsights_RespectsScheduleAndToggle(t *testing.T) {
	m, store, spy := weeklyTestManager(t)
	seedFinding(t, store, "m1", "latency_sawtooth", "m1 climbs and resets", "high")

	// Right day, too early.
	m.maybeSendWeeklyInsights(mondayAt(8, 59))
	if len(spy.events) != 0 {
		t.Errorf("sent before the configured time")
	}

	// Right time, wrong day.
	m.maybeSendWeeklyInsights(mondayAt(9, 0).AddDate(0, 0, 1))
	if len(spy.events) != 0 {
		t.Errorf("sent on the wrong weekday")
	}

	// Disabled entirely.
	m.weeklyInsights.Enabled = false
	m.maybeSendWeeklyInsights(mondayAt(9, 0))
	if len(spy.events) != 0 {
		t.Errorf("sent while disabled")
	}
}

// Nothing found is not worth a message. A weekly "no patterns this week" is one more thing
// to learn to ignore, and the daily digest already confirms Warden is alive.
func TestWeeklyInsights_StaysQuietWithNoFindings(t *testing.T) {
	m, _, spy := weeklyTestManager(t)

	m.maybeSendWeeklyInsights(mondayAt(9, 0))
	if len(spy.events) != 0 {
		t.Errorf("sent a summary with nothing to report: %+v", spy.events)
	}

	// And it does not retry all day either.
	m.maybeSendWeeklyInsights(mondayAt(9, 5))
	if len(spy.events) != 0 {
		t.Errorf("retried an empty week")
	}
}

// A reader who stops after two lines should have read the two that matter.
func TestWeeklyInsightsEvent_PutsConfidentFindingsFirst(t *testing.T) {
	findings := []db.MonitorInsight{
		{MonitorID: "m2", MonitorName: "beta", Kind: "co_failure", Summary: "beta maybe", Confidence: "medium"},
		{MonitorID: "m1", MonitorName: "alpha", Kind: "latency_sawtooth", Summary: "alpha definitely", Confidence: "high"},
	}

	ev := weeklyInsightsEvent(findings, mondayAt(9, 0))

	// Its own type, not "stabilized": a pattern summary is not a monitor state change, and
	// borrowing one would title the message "Monitor Stabilized".
	if ev.Type != notifications.EventInsights {
		t.Errorf("type = %v, want the dedicated insights type", ev.Type)
	}
	if !strings.Contains(ev.Message, "2 pattern(s) across 2 monitor(s)") {
		t.Errorf("header does not count the findings: %q", ev.Message)
	}

	confident := strings.Index(ev.Message, "alpha definitely")
	tentative := strings.Index(ev.Message, "beta maybe")
	if confident < 0 || tentative < 0 || confident > tentative {
		t.Errorf("the confident finding is not first:\n%s", ev.Message)
	}
}

func TestLoadWeeklyInsightsConfig(t *testing.T) {
	m, store := newAlertTestManager(t)

	if got := m.loadWeeklyInsightsConfig(); got != defaultWeeklyInsightsConfig() {
		t.Errorf("with no settings: %+v, want the defaults", got)
	}
	if defaultWeeklyInsightsConfig().Enabled {
		t.Error("the weekly summary must be opt-in: an upgrade should not start sending a new kind of message")
	}

	_ = store.SetSetting("notification.insights.weekly_enabled", "true")
	_ = store.SetSetting("notification.insights.weekly_day", "5")
	_ = store.SetSetting("notification.insights.weekly_time", "17:30")

	want := weeklyInsightsConfig{Enabled: true, Weekday: time.Friday, AtTime: "17:30"}
	if got := m.loadWeeklyInsightsConfig(); got != want {
		t.Errorf("loadWeeklyInsightsConfig = %+v, want %+v", got, want)
	}
}
