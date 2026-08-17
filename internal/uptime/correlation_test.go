package uptime

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/notifications"
)

func TestRequiredForGroup(t *testing.T) {
	p := defaultCorrelationPolicy() // min 3, 30%

	cases := map[int]int{
		1:   3, // the floor wins on tiny groups: one monitor is never a correlated incident
		4:   3,
		12:  4, // 30% of 12 = 3.6, rounded up
		22:  7,
		200: 60, // the percentage is what still means something at this size
	}
	for size, want := range cases {
		if got := p.requiredForGroup(size); got != want {
			t.Errorf("requiredForGroup(%d) = %d, want %d", size, got, want)
		}
	}
}

func TestCluster_AnchorsOnEarliestMember(t *testing.T) {
	base := time.Date(2026, 8, 12, 19, 29, 0, 0, time.UTC)
	mk := func(id int64, offset time.Duration) db.OpenOutage {
		return db.OpenOutage{ID: id, MonitorID: fmt.Sprintf("m%d", id), StartTime: base.Add(offset)}
	}

	// The real 12-Aug event: eleven monitors inside two minutes.
	var burst []db.OpenOutage
	for i := 0; i < 11; i++ {
		burst = append(burst, mk(int64(i), time.Duration(i*11)*time.Second))
	}
	got := cluster(burst, 5*time.Minute)
	if len(got) != 1 || len(got[0]) != 11 {
		t.Fatalf("expected one cluster of 11, got %d clusters of sizes %v", len(got), clusterSizes(got))
	}

	// A monitor that failed an hour later is a separate event, not the same one.
	withLate := append(append([]db.OpenOutage{}, burst...), mk(99, time.Hour))
	got = cluster(withLate, 5*time.Minute)
	if len(got) != 2 {
		t.Fatalf("expected 2 clusters, got %d of sizes %v", len(got), clusterSizes(got))
	}
}

// Anchoring on the earliest member is what stops a slow drip of failures from chaining
// into one enormous "incident" that spans a whole day.
func TestCluster_DoesNotChainIndefinitely(t *testing.T) {
	base := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	var drip []db.OpenOutage
	for i := 0; i < 10; i++ {
		drip = append(drip, db.OpenOutage{
			ID: int64(i), MonitorID: fmt.Sprintf("m%d", i),
			StartTime: base.Add(time.Duration(i*4) * time.Minute),
		})
	}

	got := cluster(drip, 5*time.Minute)
	if len(got) < 4 {
		t.Errorf("a 4-minute drip over 40 minutes chained into %d clusters, sizes %v", len(got), clusterSizes(got))
	}
}

func clusterSizes(cs [][]db.OpenOutage) []int {
	out := make([]int, 0, len(cs))
	for _, c := range cs {
		out = append(out, len(c))
	}
	return out
}

func TestMonitorList_CapsTheWall(t *testing.T) {
	var many []db.OpenOutage
	for i := 0; i < 11; i++ {
		many = append(many, db.OpenOutage{MonitorName: fmt.Sprintf("monitor-%02d", i)})
	}
	got := monitorList(many, 8)
	if !strings.HasSuffix(got, "and 3 more") {
		t.Errorf("expected the tail to be summarised, got %q", got)
	}

	// One monitor with both a down and a degraded row is one name, not two.
	dupes := []db.OpenOutage{{MonitorName: "API"}, {MonitorName: "API"}, {MonitorName: "Web"}}
	if got := monitorList(dupes, 8); got != "API, Web" {
		t.Errorf("monitorList did not de-duplicate: %q", got)
	}
}

func TestDistinctMonitors(t *testing.T) {
	outages := []db.OpenOutage{
		{MonitorID: "a", Type: "down"},
		{MonitorID: "a", Type: "degraded"},
		{MonitorID: "b", Type: "down"},
	}
	if got := distinctMonitors(outages); got != 2 {
		t.Errorf("distinctMonitors = %d, want 2 (one monitor broken twice is one monitor)", got)
	}
}

// --- evaluator-level behaviour ---

// newCorrelationTestManager builds a manager with `count` monitors in one group, plus a
// spy notifier, and returns everything the tests need to drive the evaluator by hand.
func newCorrelationTestManager(t *testing.T, groupID string, count int) (*Manager, *db.Store, *spyNotifier) {
	t.Helper()
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.CreateGroup(db.Group{ID: groupID, Name: "NodeSource"}); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	spy := &spyNotifier{}
	m := NewManager(store)
	m.notifier = spy

	for i := 0; i < count; i++ {
		id := fmt.Sprintf("m%02d", i)
		if err := store.CreateMonitor(db.Monitor{
			ID: id, GroupID: groupID, Name: "monitor-" + id,
			URL: "https://" + id + ".example.com", Active: true, Interval: 60,
		}); err != nil {
			t.Fatalf("CreateMonitor: %v", err)
		}
		m.monitors[id] = NewMonitor(id, db.MonitorTypeHTTP, groupID, "monitor-"+id,
			"https://"+id+".example.com", time.Minute, m.jobQueue, time.Now(), nil)
	}
	return m, store, spy
}

// addGroupWithMonitors adds a second group so a failure can span more than one of them.
func addGroupWithMonitors(t *testing.T, m *Manager, store *db.Store, groupID, name string, count int) {
	t.Helper()
	if err := store.CreateGroup(db.Group{ID: groupID, Name: name}); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("%s%02d", groupID, i)
		if err := store.CreateMonitor(db.Monitor{
			ID: id, GroupID: groupID, Name: "monitor-" + id,
			URL: "https://" + id + ".example.com", Active: true, Interval: 60,
		}); err != nil {
			t.Fatalf("CreateMonitor: %v", err)
		}
		m.monitors[id] = NewMonitor(id, db.MonitorTypeHTTP, groupID, "monitor-"+id,
			"https://"+id+".example.com", time.Minute, m.jobQueue, time.Now(), nil)
	}
}

func openOutagesFor(t *testing.T, store *db.Store, ids []string, summary string) time.Time {
	t.Helper()
	for _, id := range ids {
		if err := store.CreateOutage(id, "down", summary); err != nil {
			t.Fatalf("CreateOutage(%s): %v", id, err)
		}
	}
	open, err := store.GetOpenOutages()
	if err != nil || len(open) == 0 {
		t.Fatalf("GetOpenOutages: %v", err)
	}
	return open[0].StartTime
}

// The 12-Aug case: eleven of twelve monitors returned 404 inside two minutes. That is one
// incident, and it should read as one.
func TestEvaluateAlerts_CorrelatesGroupFailure(t *testing.T) {
	m, store, spy := newCorrelationTestManager(t, "g-ns", 12)

	var ids []string
	for i := 0; i < 11; i++ {
		ids = append(ids, fmt.Sprintf("m%02d", i))
	}
	start := openOutagesFor(t, store, ids, "Monitor is down (Status: 404)")

	m.evaluateAlerts(start.Add(3 * time.Minute))

	sent := spy.byType(notifications.EventDown)
	if len(sent) != 1 {
		t.Fatalf("expected 1 correlated alert for 11 monitors, got %d: %+v", len(sent), sent)
	}
	if !strings.Contains(sent[0].Message, "11 of 12 monitors in NodeSource") {
		t.Errorf("message does not name the scale: %q", sent[0].Message)
	}

	// Every member is stamped, or the ones left behind alert individually a tick later.
	open, _ := store.GetOpenOutages()
	for _, o := range open {
		if o.NotifiedAt == nil {
			t.Fatalf("%s was left un-stamped and will alert on its own", o.MonitorID)
		}
		if o.CorrelationID == "" || o.CorrelationID != open[0].CorrelationID {
			t.Fatalf("members do not share a correlation id: %q vs %q", o.CorrelationID, open[0].CorrelationID)
		}
	}

	m.evaluateAlerts(start.Add(4 * time.Minute))
	if got := spy.byType(notifications.EventDown); len(got) != 1 {
		t.Errorf("the correlated incident was announced %d times", len(got))
	}
}

// Below the threshold nothing is correlated: two monitors failing in a group of twelve is
// two problems until proven otherwise.
func TestEvaluateAlerts_BelowThresholdStaysIndividual(t *testing.T) {
	m, store, spy := newCorrelationTestManager(t, "g-ns", 12)
	start := openOutagesFor(t, store, []string{"m00", "m01"}, "Monitor is down")

	m.evaluateAlerts(start.Add(3 * time.Minute))

	sent := spy.byType(notifications.EventDown)
	if len(sent) != 2 {
		t.Fatalf("expected 2 individual alerts, got %d: %+v", len(sent), sent)
	}
	for _, e := range sent {
		if e.MonitorID == "" {
			t.Errorf("individual alert lost its monitor identity: %+v", e)
		}
	}
}

// When everything is down across unrelated groups, the likely culprit is Warden's own
// network, and saying so beats messages blaming each innocent service.
func TestEvaluateAlerts_ProbeWideFailure(t *testing.T) {
	m, store, spy := newCorrelationTestManager(t, "g-ns", 6)
	addGroupWithMonitors(t, m, store, "g-other", "Homelab", 4)

	var ids []string
	for i := 0; i < 6; i++ {
		ids = append(ids, fmt.Sprintf("m%02d", i))
	}
	for i := 0; i < 3; i++ {
		ids = append(ids, fmt.Sprintf("g-other%02d", i))
	}
	start := openOutagesFor(t, store, ids, "Monitor is down")

	m.evaluateAlerts(start.Add(3 * time.Minute))

	sent := spy.byType(notifications.EventDown)
	if len(sent) != 1 {
		t.Fatalf("expected 1 probe-wide alert, got %d: %+v", len(sent), messages(sent))
	}
	if !strings.Contains(sent[0].Message, "Warden's own network") {
		t.Errorf("the probe-wide alert should point at the probe, got %q", sent[0].Message)
	}

	open, _ := store.GetOpenOutages()
	for _, o := range open {
		if o.NotifiedAt == nil || !strings.HasPrefix(o.CorrelationID, "probe-") {
			t.Fatalf("%s not folded into the probe incident: notified=%v corr=%q",
				o.MonitorID, o.NotifiedAt, o.CorrelationID)
		}
	}
}

// A single group failing entirely looks identical to a dead probe from here. The group
// explanation is more specific and safer to be wrong about, so it wins.
func TestEvaluateAlerts_WholeSingleGroupIsNotBlamedOnTheProbe(t *testing.T) {
	m, store, spy := newCorrelationTestManager(t, "g-ns", 6)

	var ids []string
	for i := 0; i < 6; i++ {
		ids = append(ids, fmt.Sprintf("m%02d", i))
	}
	start := openOutagesFor(t, store, ids, "Monitor is down")

	m.evaluateAlerts(start.Add(3 * time.Minute))

	sent := spy.byType(notifications.EventDown)
	if len(sent) != 1 {
		t.Fatalf("expected 1 correlated alert, got %d: %+v", len(sent), messages(sent))
	}
	if strings.Contains(sent[0].Message, "Warden's own network") {
		t.Errorf("one group down should not be blamed on the probe: %q", sent[0].Message)
	}
	if !strings.Contains(sent[0].Message, "6 of 6 monitors in NodeSource") {
		t.Errorf("expected a group-shaped message, got %q", sent[0].Message)
	}
}

// A monitor that has already interrupted you three times today is describing itself, not
// an event. It says that once, then goes quiet without going invisible.
func TestEvaluateAlerts_ChronicOffenderIsDamped(t *testing.T) {
	m, store, spy := newCorrelationTestManager(t, "g-ns", 12)
	base := time.Now().UTC().Add(-2 * time.Hour)

	// Two episodes already announced today.
	for i := 0; i < 2; i++ {
		if err := store.CreateOutage("m00", "down", "Monitor is down"); err != nil {
			t.Fatalf("CreateOutage: %v", err)
		}
		open, _ := store.GetOpenOutages()
		if _, err := store.MarkOutageNotified(open[0].ID, base.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatalf("MarkOutageNotified: %v", err)
		}
		if err := store.CloseOutage("m00"); err != nil {
			t.Fatalf("CloseOutage: %v", err)
		}
	}

	// The third crosses the line and says so.
	start := openOutagesFor(t, store, []string{"m00"}, "Monitor is down")
	m.evaluateAlerts(start.Add(3 * time.Minute))

	notices := spy.byType(notifications.EventFlapping)
	if len(notices) != 1 {
		t.Fatalf("expected 1 'unstable' notice, got %d: %+v", len(notices), notices)
	}
	if !strings.Contains(notices[0].Message, "Muting its individual alerts") {
		t.Errorf("the notice should say what it is about to do: %q", notices[0].Message)
	}
	if got := spy.byType(notifications.EventDown); len(got) != 0 {
		t.Errorf("the crossing episode should not also send a normal alert: %+v", got)
	}
	if err := store.CloseOutage("m00"); err != nil {
		t.Fatalf("CloseOutage: %v", err)
	}

	// The fourth is silent, and stays un-stamped so it produces no reminder and no
	// recovery message either.
	start = openOutagesFor(t, store, []string{"m00"}, "Monitor is down")
	m.evaluateAlerts(start.Add(3 * time.Minute))

	if got := spy.byType(notifications.EventDown); len(got) != 0 {
		t.Errorf("a damped monitor still alerted: %+v", got)
	}
	if got := spy.byType(notifications.EventFlapping); len(got) != 1 {
		t.Errorf("the 'unstable' notice repeated: %d", len(got))
	}
	open, _ := store.GetOpenOutages()
	if open[0].NotifiedAt != nil {
		t.Error("a damped outage must stay un-stamped, so its recovery stays quiet too")
	}
}

// A muted monitor keeps recording outages and keeps appearing in the digest. It just never
// interrupts anyone — the point of the flag for test and staging targets.
func TestEvaluateAlerts_MutedMonitorNeverAlerts(t *testing.T) {
	m, store, spy := newCorrelationTestManager(t, "g-ns", 12)
	if err := store.SetMonitorAlertsMuted("m00", true); err != nil {
		t.Fatalf("SetMonitorAlertsMuted: %v", err)
	}
	start := openOutagesFor(t, store, []string{"m00"}, "Monitor is down")

	m.evaluateAlerts(start.Add(3 * time.Hour))

	if got := spy.byType(notifications.EventDown); len(got) != 0 {
		t.Errorf("a muted monitor alerted: %+v", got)
	}
	open, _ := store.GetOpenOutages()
	if len(open) != 1 {
		t.Fatal("the outage should still be recorded")
	}
	if open[0].NotifiedAt != nil {
		t.Error("a muted monitor must not be stamped as announced")
	}
}

// Eleven monitors that failed as one event must not turn into eleven reminders half an
// hour later.
func TestEvaluateAlerts_CorrelatedIncidentRemindsOnce(t *testing.T) {
	m, store, spy := newCorrelationTestManager(t, "g-ns", 12)

	var ids []string
	for i := 0; i < 11; i++ {
		ids = append(ids, fmt.Sprintf("m%02d", i))
	}
	start := openOutagesFor(t, store, ids, "Monitor is down (Status: 404)")

	m.evaluateAlerts(start.Add(3 * time.Minute))
	m.evaluateAlerts(start.Add(34 * time.Minute))

	sent := spy.byType(notifications.EventDown)
	if len(sent) != 2 {
		t.Fatalf("expected the alert plus one reminder, got %d: %+v", len(sent), messages(sent))
	}
	if !strings.HasPrefix(sent[1].Message, "Still down after") {
		t.Errorf("second message is not a reminder: %q", sent[1].Message)
	}
}

func messages(evs []notifications.NotificationEvent) []string {
	out := make([]string, 0, len(evs))
	for _, e := range evs {
		out = append(out, e.Message)
	}
	return out
}

func TestLoadCorrelationPolicy_ReadsSettings(t *testing.T) {
	m, store, _ := newCorrelationTestManager(t, "g-ns", 1)

	if got := m.loadCorrelationPolicy(); got != defaultCorrelationPolicy() {
		t.Errorf("with no settings: %+v, want the defaults", got)
	}

	_ = store.SetSetting("notification.correlation.window_seconds", "600")
	_ = store.SetSetting("notification.correlation.min_monitors", "5")
	_ = store.SetSetting("notification.correlation.group_percent", "50")
	_ = store.SetSetting("notification.correlation.probe_percent", "90")
	_ = store.SetSetting("notification.chronic.limit", "0")
	_ = store.SetSetting("notification.chronic.window_minutes", "720")

	want := correlationPolicy{
		Window:        10 * time.Minute,
		MinMonitors:   5,
		GroupPercent:  50,
		ProbePercent:  90,
		ChronicLimit:  0,
		ChronicWindow: 12 * time.Hour,
	}
	if got := m.loadCorrelationPolicy(); got != want {
		t.Errorf("loadCorrelationPolicy = %+v, want %+v", got, want)
	}
}

// Setting the chronic limit to 0 turns the damping off rather than muting everything.
func TestEvaluateAlerts_ChronicLimitZeroDisablesDamping(t *testing.T) {
	m, store, spy := newCorrelationTestManager(t, "g-ns", 12)
	m.correlationPolicy.ChronicLimit = 0

	for i := 0; i < 4; i++ {
		start := openOutagesFor(t, store, []string{"m00"}, "Monitor is down")
		m.evaluateAlerts(start.Add(3 * time.Minute))
		if err := store.CloseOutage("m00"); err != nil {
			t.Fatalf("CloseOutage: %v", err)
		}
	}

	if got := spy.byType(notifications.EventDown); len(got) != 4 {
		t.Errorf("with damping off all 4 episodes should alert, got %d", len(got))
	}
	if got := spy.byType(notifications.EventFlapping); len(got) != 0 {
		t.Errorf("with damping off there should be no 'unstable' notice: %+v", got)
	}
}
