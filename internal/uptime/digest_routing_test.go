package uptime

import (
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/notifications"
)

func digestRoutingManager(t *testing.T, digestTypes string) (*Manager, *db.Store, *spyNotifier) {
	t.Helper()
	m, store := newAlertTestManager(t)
	spy := &spyNotifier{}
	m.notifier = spy

	if err := store.SetSetting("notification.digest.enabled", "true"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if err := store.SetSetting("notification.digest.event_types", digestTypes); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	m.Sync()
	return m, store, spy
}

func digestEventTypes(t *testing.T, store *db.Store) []string {
	t.Helper()
	events, err := store.GetUnsentDigestEvents()
	if err != nil {
		t.Fatalf("GetUnsentDigestEvents: %v", err)
	}
	out := make([]string, 0, len(events))
	for _, e := range events {
		out = append(out, e.EventType)
	}
	return out
}

func downEvent() notifications.NotificationEvent {
	return notifications.NotificationEvent{
		MonitorID: "m1", MonitorName: "Jellyfin", MonitorURL: "https://jellyfin.example.com",
		Type: notifications.EventDown, Message: "Monitor is down (Status: 503)", Time: time.Now(),
	}
}

// The central claim of this phase: selecting an event for the digest no longer diverts it.
// This is the regression test for the footgun that left an operator weeks without
// notifications after choosing "down" as a digest event.
func TestArchiveAndNotifyAreIndependent(t *testing.T) {
	m, store, _ := digestRoutingManager(t, "down,degraded")

	m.archiveForDigest(downEvent())

	if got := digestEventTypes(t, store); len(got) != 1 || got[0] != "down" {
		t.Fatalf("a down event selected for the digest was not archived: %v", got)
	}
}

// Archiving is scoped by the digest selection, so an event the operator did not pick does
// not silently appear in the summary.
func TestArchiveForDigest_RespectsTheSelection(t *testing.T) {
	m, store, _ := digestRoutingManager(t, "degraded,flapping")

	m.archiveForDigest(downEvent())

	if got := digestEventTypes(t, store); len(got) != 0 {
		t.Errorf("an event outside the digest selection was archived anyway: %v", got)
	}
}

// With the digest off, nothing is archived at all — and, crucially, nothing is lost from
// the immediate path either, which is what the split guarantees.
func TestArchiveForDigest_NoOpWhenDigestIsOff(t *testing.T) {
	m, store := newAlertTestManager(t)
	if err := store.SetSetting("notification.digest.enabled", "false"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	m.Sync()

	m.archiveForDigest(downEvent())

	if got := digestEventTypes(t, store); len(got) != 0 {
		t.Errorf("events were archived with the digest disabled: %v", got)
	}
}

// recordEvent is the path for the one-shot events. It must archive and notify as two
// separate decisions rather than one choosing between them.
func TestRecordEvent_ArchivesAndNotifiesIndependently(t *testing.T) {
	m, store, spy := digestRoutingManager(t, "flapping")

	flap := notifications.NotificationEvent{
		MonitorID: "m1", MonitorName: "Jellyfin",
		Type: notifications.EventFlapping, Message: "Monitor is flapping between states",
		Time: time.Now(),
	}

	// In the digest *and* enabled: both happen. Under the old routing the immediate send
	// would have been skipped entirely.
	m.recordEvent(flap, true)
	if got := digestEventTypes(t, store); len(got) != 1 {
		t.Errorf("expected 1 archived event, got %v", got)
	}
	if got := spy.byType(notifications.EventFlapping); len(got) != 1 {
		t.Errorf("expected 1 immediate notification, got %d", len(got))
	}

	// Turning the immediate side off leaves the archive untouched: the summary still knows
	// it happened.
	m.recordEvent(flap, false)
	if got := digestEventTypes(t, store); len(got) != 2 {
		t.Errorf("expected the second event to still be archived, got %v", got)
	}
	if got := spy.byType(notifications.EventFlapping); len(got) != 1 {
		t.Errorf("an event with notify=false was sent anyway: %d", len(got))
	}
}

// A down outage is archived at the moment it is confirmed, not when it is announced: the
// day's summary should reflect when the monitor actually broke, even though the alert is
// held back for the sustained window.
func TestDownIsArchivedAtTheTransitionNotTheAlert(t *testing.T) {
	m, store, spy := digestRoutingManager(t, "down")

	m.archiveForDigest(downEvent())

	if got := digestEventTypes(t, store); len(got) != 1 {
		t.Fatalf("the outage was not archived when it was confirmed: %v", got)
	}
	if got := spy.byType(notifications.EventDown); len(got) != 0 {
		t.Errorf("archiving also notified — the two are supposed to be separate: %+v", got)
	}
}

// closeOutageAndAnnounce carries two independent conditions: whether the outage had been
// announced, and whether the operator wants recovery messages at all (maintenance, the
// flapping guard, the "up" toggle). Both have to hold.
func TestCloseOutageAndAnnounce_BothConditions(t *testing.T) {
	recovery := func() notifications.NotificationEvent {
		return notifications.NotificationEvent{
			MonitorID: "m1", MonitorName: "Jellyfin",
			Type: notifications.EventUp, Message: "Monitor Recovered", Time: time.Now(),
		}
	}

	waitForClose := func(t *testing.T, store *db.Store) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if open, _ := store.GetOpenOutages(); len(open) == 0 {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("the outage never closed")
	}

	t.Run("announced and allowed sends", func(t *testing.T) {
		m, store := newAlertTestManager(t)
		spy := &spyNotifier{}
		m.notifier = spy

		if err := store.CreateOutage("m1", "down", "Monitor is down"); err != nil {
			t.Fatalf("CreateOutage: %v", err)
		}
		open, _ := store.GetOpenOutages()
		if _, err := store.MarkOutageNotified(open[0].ID, time.Now().UTC()); err != nil {
			t.Fatalf("MarkOutageNotified: %v", err)
		}

		m.closeOutageAndAnnounce("m1", recovery(), true)
		waitForClose(t, store)

		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) && len(spy.byType(notifications.EventUp)) == 0 {
			time.Sleep(20 * time.Millisecond)
		}
		if got := spy.byType(notifications.EventUp); len(got) != 1 {
			t.Errorf("an announced outage that recovered sent %d recovery messages, want 1", len(got))
		}
	})

	t.Run("announced but not allowed stays quiet", func(t *testing.T) {
		m, store := newAlertTestManager(t)
		spy := &spyNotifier{}
		m.notifier = spy

		if err := store.CreateOutage("m1", "down", "Monitor is down"); err != nil {
			t.Fatalf("CreateOutage: %v", err)
		}
		open, _ := store.GetOpenOutages()
		if _, err := store.MarkOutageNotified(open[0].ID, time.Now().UTC()); err != nil {
			t.Fatalf("MarkOutageNotified: %v", err)
		}

		// allowed=false is what maintenance, flapping and a disabled "up" toggle produce.
		m.closeOutageAndAnnounce("m1", recovery(), false)
		waitForClose(t, store)
		time.Sleep(200 * time.Millisecond)

		if got := spy.byType(notifications.EventUp); len(got) != 0 {
			t.Errorf("recovery sent while suppressed: %+v", got)
		}
	})

	t.Run("never announced stays quiet even when allowed", func(t *testing.T) {
		m, store := newAlertTestManager(t)
		spy := &spyNotifier{}
		m.notifier = spy

		if err := store.CreateOutage("m1", "down", "Monitor is down"); err != nil {
			t.Fatalf("CreateOutage: %v", err)
		}

		m.closeOutageAndAnnounce("m1", recovery(), true)
		waitForClose(t, store)
		time.Sleep(200 * time.Millisecond)

		if got := spy.byType(notifications.EventUp); len(got) != 0 {
			t.Errorf("recovery sent for an outage nobody was told about: %+v", got)
		}
	})
}
