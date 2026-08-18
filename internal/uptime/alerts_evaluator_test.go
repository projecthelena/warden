package uptime

import (
	"sync"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/notifications"
)

// newAlertTestManager builds a manager wired to a store, with one registered monitor, and
// without starting any goroutine: the evaluator is driven by hand so the clock is ours.
func newAlertTestManager(t *testing.T) (*Manager, *db.Store) {
	t.Helper()
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.CreateGroup(db.Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if err := store.CreateMonitor(db.Monitor{
		ID: "m1", GroupID: "g1", Name: "Jellyfin",
		URL: "https://jellyfin.example.com", Active: true, Interval: 60,
	}); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}

	m := NewManager(store)
	mon := NewMonitor("m1", db.MonitorTypeHTTP, "g1", "Jellyfin", "https://jellyfin.example.com",
		time.Minute, m.jobQueue, time.Now(), nil)
	m.monitors["m1"] = mon

	return m, store
}

func openOutage(t *testing.T, store *db.Store, kind, summary string) db.OpenOutage {
	t.Helper()
	if err := store.CreateOutage("m1", kind, summary); err != nil {
		t.Fatalf("CreateOutage: %v", err)
	}
	open, err := store.GetOpenOutages()
	if err != nil || len(open) == 0 {
		t.Fatalf("GetOpenOutages: %v (%d rows)", err, len(open))
	}
	return open[0]
}

func notifiedAt(t *testing.T, store *db.Store) *time.Time {
	t.Helper()
	open, err := store.GetOpenOutages()
	if err != nil {
		t.Fatalf("GetOpenOutages: %v", err)
	}
	if len(open) == 0 {
		return nil
	}
	return open[0].NotifiedAt
}

// The headline behaviour: a monitor that just went down is not announced, and the same
// monitor still down three minutes later is.
func TestEvaluateAlerts_SilentThenAnnounced(t *testing.T) {
	m, store := newAlertTestManager(t)
	o := openOutage(t, store, "down", "Monitor is down (Status: 503)")

	m.evaluateAlerts(o.StartTime.Add(2 * time.Minute))
	if got := notifiedAt(t, store); got != nil {
		t.Fatalf("outage announced after 2m, want silence (notified_at=%v)", got)
	}

	m.evaluateAlerts(o.StartTime.Add(3 * time.Minute))
	if notifiedAt(t, store) == nil {
		t.Fatal("outage still silent after 3m, want an alert")
	}
}

func TestEvaluateAlerts_AnnouncesOnce(t *testing.T) {
	m, store := newAlertTestManager(t)
	o := openOutage(t, store, "down", "Monitor is down")

	m.evaluateAlerts(o.StartTime.Add(3 * time.Minute))
	first := notifiedAt(t, store)
	if first == nil {
		t.Fatal("no alert on the first eligible tick")
	}

	// Every subsequent tick before the reminder is due must leave the stamp alone.
	for _, d := range []time.Duration{4, 10, 20} {
		m.evaluateAlerts(o.StartTime.Add(d * time.Minute))
	}
	again := notifiedAt(t, store)
	if again == nil || !again.Equal(*first) {
		t.Errorf("notified_at moved from %v to %v — the outage was announced more than once", first, again)
	}
}

func TestEvaluateAlerts_SkipsMonitorInMaintenance(t *testing.T) {
	m, store := newAlertTestManager(t)
	o := openOutage(t, store, "down", "Monitor is down")

	now := time.Now().UTC()
	m.maintenanceWindows = []db.Incident{{
		StartTime:      now.Add(-time.Hour),
		AffectedGroups: `["g1"]`,
	}}

	m.evaluateAlerts(o.StartTime.Add(10 * time.Minute))
	if got := notifiedAt(t, store); got != nil {
		t.Errorf("alerted during a maintenance window (notified_at=%v)", got)
	}
}

func TestEvaluateAlerts_SkipsDisabledEventType(t *testing.T) {
	m, store := newAlertTestManager(t)
	o := openOutage(t, store, "down", "Monitor is down")

	m.eventFilter.DownEnabled = false

	m.evaluateAlerts(o.StartTime.Add(10 * time.Minute))
	if got := notifiedAt(t, store); got != nil {
		t.Errorf("alerted with down notifications turned off (notified_at=%v)", got)
	}
}

// A paused monitor keeps its outage open on purpose, but nobody is checking it, so it has
// nothing new to say. Without this guard a monitor paused mid-outage alerts forever.
func TestEvaluateAlerts_SkipsUnregisteredMonitor(t *testing.T) {
	m, store := newAlertTestManager(t)
	o := openOutage(t, store, "down", "Monitor is down")

	delete(m.monitors, "m1")

	m.evaluateAlerts(o.StartTime.Add(10 * time.Minute))
	if got := notifiedAt(t, store); got != nil {
		t.Errorf("alerted for a monitor that is not running (notified_at=%v)", got)
	}
}

func TestEvaluateAlerts_RemindersFollowTheLadder(t *testing.T) {
	m, store := newAlertTestManager(t)
	o := openOutage(t, store, "down", "Monitor is down")

	m.evaluateAlerts(o.StartTime.Add(3 * time.Minute))

	m.evaluateAlerts(o.StartTime.Add(20 * time.Minute))
	open, _ := store.GetOpenOutages()
	if open[0].LastReminderAt != nil {
		t.Fatal("reminded before the first reminder was due")
	}

	m.evaluateAlerts(o.StartTime.Add(34 * time.Minute))
	open, _ = store.GetOpenOutages()
	if open[0].LastReminderAt == nil {
		t.Fatal("no reminder 30m after the alert")
	}
	firstReminder := *open[0].LastReminderAt

	// The next reminder waits a full hour, not another 30 minutes.
	m.evaluateAlerts(o.StartTime.Add(80 * time.Minute))
	open, _ = store.GetOpenOutages()
	if !open[0].LastReminderAt.Equal(firstReminder) {
		t.Error("reminded again before the repeat cadence was due")
	}

	m.evaluateAlerts(o.StartTime.Add(95 * time.Minute))
	open, _ = store.GetOpenOutages()
	if open[0].LastReminderAt.Equal(firstReminder) {
		t.Error("no reminder once the repeat cadence came due")
	}
}

func TestEvaluateAlerts_DegradedFollowsTheSameLadder(t *testing.T) {
	m, store := newAlertTestManager(t)
	o := openOutage(t, store, "degraded", "High latency detected (>1000ms)")

	m.evaluateAlerts(o.StartTime.Add(2 * time.Minute))
	if got := notifiedAt(t, store); got != nil {
		t.Fatalf("degraded announced after 2m, want silence (notified_at=%v)", got)
	}

	m.evaluateAlerts(o.StartTime.Add(3 * time.Minute))
	if notifiedAt(t, store) == nil {
		t.Error("degraded still silent after 3m")
	}
}

func TestLoadAlertPolicy_ReadsSettings(t *testing.T) {
	m, store := newAlertTestManager(t)

	if got := m.loadAlertPolicy(); got != defaultAlertPolicy() {
		t.Errorf("with no settings: %+v, want the defaults", got)
	}

	_ = store.SetSetting("notification.alert.sustained_seconds", "300")
	_ = store.SetSetting("notification.alert.reminder_minutes", "15")
	_ = store.SetSetting("notification.alert.repeat_reminder_minutes", "120")

	want := alertPolicy{
		SustainedFor:        5 * time.Minute,
		FirstReminderAfter:  15 * time.Minute,
		RepeatReminderAfter: 2 * time.Hour,
	}
	if got := m.loadAlertPolicy(); got != want {
		t.Errorf("loadAlertPolicy = %+v, want %+v", got, want)
	}

	// Garbage and negatives leave the default in place rather than disabling alerting.
	_ = store.SetSetting("notification.alert.sustained_seconds", "-1")
	if got := m.loadAlertPolicy(); got.SustainedFor != defaultAlertPolicy().SustainedFor {
		t.Errorf("negative sustained_seconds took effect: %v", got.SustainedFor)
	}
	_ = store.SetSetting("notification.alert.sustained_seconds", "banana")
	if got := m.loadAlertPolicy(); got.SustainedFor != defaultAlertPolicy().SustainedFor {
		t.Errorf("unparseable sustained_seconds took effect: %v", got.SustainedFor)
	}

	// Zero is a real choice: alert the moment the outage opens.
	_ = store.SetSetting("notification.alert.sustained_seconds", "0")
	if got := m.loadAlertPolicy(); got.SustainedFor != 0 {
		t.Errorf("sustained_seconds=0 should mean no silent window, got %v", got.SustainedFor)
	}
}

// --- evaluator edge cases ---

// A monitor deleted mid-flight takes its outages with it (foreign key cascade), so the
// evaluator simply finds nothing. The guard that matters is that it does not panic on the
// nil monitor it would otherwise look up.
func TestEvaluateAlerts_HandlesAnEmptyWorld(t *testing.T) {
	m, _ := newAlertTestManager(t)

	// No outages at all: the evaluator must return without touching the notifier.
	m.evaluateAlerts(time.Now())

	// And with the monitor gone from the running set but an outage still open.
	openOutage(t, m.store, "down", "Monitor is down")
	delete(m.monitors, "m1")
	m.evaluateAlerts(time.Now().Add(time.Hour))
}

// The evaluator holds no lock while it works, and Sync can replace the policy underneath
// it. Running both concurrently is the cheapest way to catch a lock that was forgotten.
func TestEvaluateAlerts_IsSafeAlongsideSync(t *testing.T) {
	m, store := newAlertTestManager(t)
	openOutage(t, store, "down", "Monitor is down")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			m.evaluateAlerts(time.Now().Add(time.Hour))
		}()
		go func() {
			defer wg.Done()
			m.Sync()
		}()
	}
	wg.Wait()
}

// Two ticks landing on the same outage at once must produce one alert, not two. The claim
// is exclusive at the database, and this is the test that says so from the evaluator's
// side rather than the store's.
func TestEvaluateAlerts_ConcurrentTicksAnnounceOnce(t *testing.T) {
	m, store := newAlertTestManager(t)
	spy := &spyNotifier{}
	m.notifier = spy
	o := openOutage(t, store, "down", "Monitor is down")

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.evaluateAlerts(o.StartTime.Add(3 * time.Minute))
		}()
	}
	wg.Wait()

	if got := spy.byType(notifications.EventDown); len(got) != 1 {
		t.Errorf("concurrent ticks announced the same outage %d times", len(got))
	}
}

// An outage that opened during maintenance and is still open after the window closes is
// real and unannounced, so it must be announced then — the guard suppresses, it does not
// consume the outage.
func TestEvaluateAlerts_AnnouncesOnceMaintenanceEnds(t *testing.T) {
	m, store := newAlertTestManager(t)
	spy := &spyNotifier{}
	m.notifier = spy
	o := openOutage(t, store, "down", "Monitor is down")

	now := time.Now().UTC()
	ends := now.Add(30 * time.Minute)
	m.maintenanceWindows = []db.Incident{{
		StartTime: now.Add(-time.Hour), EndTime: &ends, AffectedGroups: `["g1"]`,
	}}

	m.evaluateAlerts(o.StartTime.Add(5 * time.Minute))
	if got := spy.byType(notifications.EventDown); len(got) != 0 {
		t.Fatalf("alerted during maintenance: %+v", got)
	}

	// The window closes; the outage is still open and still un-announced.
	m.maintenanceWindows = nil
	m.evaluateAlerts(o.StartTime.Add(40 * time.Minute))
	if got := spy.byType(notifications.EventDown); len(got) != 1 {
		t.Errorf("expected the outage to be announced after maintenance, got %d alerts", len(got))
	}
}

// A flapping monitor is suppressed by design, and must resume being alertable once it
// settles rather than staying silent for the life of the outage.
func TestEvaluateAlerts_ResumesAfterFlappingSettles(t *testing.T) {
	m, store := newAlertTestManager(t)
	spy := &spyNotifier{}
	m.notifier = spy
	o := openOutage(t, store, "down", "Monitor is down")

	mon := m.monitors["m1"]
	mon.mu.Lock()
	mon.isFlapping = true
	mon.mu.Unlock()

	m.evaluateAlerts(o.StartTime.Add(5 * time.Minute))
	if got := spy.byType(notifications.EventDown); len(got) != 0 {
		t.Fatalf("alerted while flapping: %+v", got)
	}

	mon.mu.Lock()
	mon.isFlapping = false
	mon.mu.Unlock()

	m.evaluateAlerts(o.StartTime.Add(10 * time.Minute))
	if got := spy.byType(notifications.EventDown); len(got) != 1 {
		t.Errorf("expected the alert once it settled, got %d", len(got))
	}
}
