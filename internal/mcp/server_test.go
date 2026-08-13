package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/uptime"
)

var testDBCounter atomic.Int64

// fakeWriter stands in for the API handlers, so these tests exercise the tool layer
// without dragging the HTTP package in.
type fakeWriter struct {
	store *db.Store
}

func (w *fakeWriter) AddMonitor(in MonitorInput) (db.Monitor, error) {
	m := db.Monitor{ID: "m-" + in.Name, Type: db.NormalizeMonitorType(in.Type), Name: in.Name, URL: in.URL, GroupID: in.GroupID, Interval: in.Interval, Active: true}
	return m, w.store.CreateMonitor(m)
}

func (w *fakeWriter) AddGroup(name string) (db.Group, error) {
	g := db.Group{ID: "g-" + name, Name: name}
	return g, w.store.CreateGroup(g)
}

func (w *fakeWriter) SetMonitorActive(id string, active bool) error {
	return w.store.SetMonitorActive(id, active)
}

func (w *fakeWriter) RenameGroup(id, name string) error { return w.store.UpdateGroup(id, name) }

func newTestServer(t *testing.T) (*Server, *db.Store) {
	t.Helper()

	n := testDBCounter.Add(1)
	store, err := db.NewStore(db.NewTestConfigWithPath(fmt.Sprintf("file:mcp_%d?mode=memory&cache=shared", n)))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	manager := uptime.NewManager(store)
	t.Cleanup(manager.Reset)

	writable := func(*http.Request) bool { return true }
	return NewServer(store, manager, "test", &fakeWriter{store: store}, writable), store
}

func seedMonitor(t *testing.T, store *db.Store, id, name, url string) {
	t.Helper()
	if err := store.CreateMonitor(db.Monitor{
		ID: id, GroupID: "g-default", Name: name, URL: url, Active: true, Interval: 60,
	}); err != nil {
		t.Fatalf("failed to seed monitor %s: %v", id, err)
	}
}

// An assistant asked about "Google" has no way to know the id is m-google-a1b2c3, so the
// tools have to take either.
func TestResolveMonitorByNameOrID(t *testing.T) {
	s, store := newTestServer(t)
	seedMonitor(t, store, "m-google-abc", "Google", "https://google.com")

	for _, query := range []string{"Google", "google", "  Google  ", "m-google-abc"} {
		m, err := s.resolveMonitor(query)
		if err != nil {
			t.Errorf("resolveMonitor(%q) failed: %v", query, err)
			continue
		}
		if m.ID != "m-google-abc" {
			t.Errorf("resolveMonitor(%q) returned %q", query, m.ID)
		}
	}
}

// A wrong name is the most likely mistake, so the error has to help rather than just say no.
func TestResolveMonitorUnknownListsWhatExists(t *testing.T) {
	s, store := newTestServer(t)
	seedMonitor(t, store, "m-api", "Production API", "https://api.example.com")

	_, err := s.resolveMonitor("Prod API")
	if err == nil {
		t.Fatal("expected an error for an unknown monitor")
	}
	if !strings.Contains(err.Error(), "Production API") {
		t.Errorf("expected the error to list the known monitors, got %q", err)
	}
}

func TestResolveMonitorRejectsEmpty(t *testing.T) {
	s, _ := newTestServer(t)
	if _, err := s.resolveMonitor("   "); err == nil {
		t.Fatal("expected an error for an empty monitor argument")
	}
}

func TestListMonitorsReportsStatusAndCounts(t *testing.T) {
	s, store := newTestServer(t)
	seedMonitor(t, store, "m-1", "One", "https://one.example.com")
	seedMonitor(t, store, "m-2", "Two", "https://two.example.com")

	_, out, err := s.listMonitors(context.Background(), nil, ListMonitorsInput{})
	if err != nil {
		t.Fatalf("list_monitors failed: %v", err)
	}
	if out.Total != 2 || len(out.Monitors) != 2 {
		t.Fatalf("expected 2 monitors, got total=%d listed=%d", out.Total, len(out.Monitors))
	}
	// Nothing has been checked yet, so the manager holds no state for them.
	for _, m := range out.Monitors {
		if m.Status != "unknown" {
			t.Errorf("%s: expected unknown before the first check, got %q", m.Name, m.Status)
		}
		if m.Group != "Default" {
			t.Errorf("%s: expected the group name to be resolved, got %q", m.Name, m.Group)
		}
	}
}

func TestListMonitorsFiltersByStatus(t *testing.T) {
	s, store := newTestServer(t)
	seedMonitor(t, store, "m-1", "Running", "https://one.example.com")
	seedMonitor(t, store, "m-2", "Paused", "https://two.example.com")
	if err := store.SetMonitorActive("m-2", false); err != nil {
		t.Fatalf("failed to pause monitor: %v", err)
	}

	_, out, err := s.listMonitors(context.Background(), nil, ListMonitorsInput{Status: "paused"})
	if err != nil {
		t.Fatalf("list_monitors failed: %v", err)
	}
	if len(out.Monitors) != 1 || out.Monitors[0].Name != "Paused" {
		t.Fatalf("expected only the paused monitor, got %+v", out.Monitors)
	}
	// The filter narrows the list but the totals still describe everything.
	if out.Total != 2 {
		t.Errorf("expected the total to count every monitor, got %d", out.Total)
	}
}

func TestGetMonitorEventsClampsLimit(t *testing.T) {
	s, store := newTestServer(t)
	seedMonitor(t, store, "m-1", "Noisy", "https://one.example.com")
	for i := 0; i < 5; i++ {
		if err := store.CreateEvent("m-1", "down", "Monitor is down"); err != nil {
			t.Fatalf("failed to seed event: %v", err)
		}
	}

	_, out, err := s.getMonitorEvents(context.Background(), nil, GetMonitorEventsInput{
		Monitor: "Noisy",
		Limit:   10_000,
	})
	if err != nil {
		t.Fatalf("get_monitor_events failed: %v", err)
	}
	if len(out.Events) > maxEvents {
		t.Errorf("expected the limit to be clamped to %d, got %d events", maxEvents, len(out.Events))
	}
	if out.Monitor != "Noisy" {
		t.Errorf("expected the monitor name in the output, got %q", out.Monitor)
	}
}

func TestIncidentSummaryMarksOngoing(t *testing.T) {
	start := time.Now().Add(-90 * time.Second)
	ongoing := incidentSummary(db.MonitorOutage{MonitorName: "API", Type: "down", StartTime: start})
	if !ongoing.Ongoing || ongoing.Ended != "" {
		t.Errorf("expected an open outage to be marked ongoing, got %+v", ongoing)
	}

	end := start.Add(60 * time.Second)
	closed := incidentSummary(db.MonitorOutage{MonitorName: "API", Type: "down", StartTime: start, EndTime: &end})
	if closed.Ongoing {
		t.Error("expected a closed outage not to be marked ongoing")
	}
	if closed.Duration != "1m0s" {
		t.Errorf("expected a 1 minute duration, got %q", closed.Duration)
	}
}

// The protocol handshake is the part most likely to break on an SDK upgrade, so drive it
// with a real client over the real HTTP handler rather than calling the tools directly.
func TestMCPClientCanListAndCallTools(t *testing.T) {
	s, store := newTestServer(t)
	seedMonitor(t, store, "m-api", "Production API", "https://api.example.com")

	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.URL}, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = session.Close() }()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("tools/list failed: %v", err)
	}
	found := make(map[string]bool, len(tools.Tools))
	for _, tool := range tools.Tools {
		found[tool.Name] = true
	}
	for _, want := range []string{"list_monitors", "get_monitor", "list_incidents", "get_monitor_events"} {
		if !found[want] {
			t.Errorf("expected tool %q to be advertised", want)
		}
	}

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_monitor",
		Arguments: map[string]any{"monitor": "Production API"},
	})
	if err != nil {
		t.Fatalf("tools/call failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool reported an error: %+v", res.Content)
	}
}

// A bad argument should come back as a tool error the model can read and correct, not as
// a transport failure that kills the session.
func TestUnknownMonitorIsAToolErrorNotATransportError(t *testing.T) {
	s, store := newTestServer(t)
	seedMonitor(t, store, "m-api", "Production API", "https://api.example.com")

	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.URL}, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = session.Close() }()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_monitor",
		Arguments: map[string]any{"monitor": "Nope"},
	})
	if err != nil {
		t.Fatalf("expected a tool error, got a transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected the call to be reported as a tool error")
	}
}

func readOnlyServer(t *testing.T) *Server {
	t.Helper()
	s, _ := newTestServer(t)
	s.canWrite = func(*http.Request) bool { return false }
	return s
}

// A viewer key must not even be offered the write tools: a model that cannot see them
// cannot propose an action the server is going to refuse.
func TestViewerNeverSeesWriteTools(t *testing.T) {
	srv := httptest.NewServer(readOnlyServer(t).Handler())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.URL}, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = session.Close() }()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("tools/list failed: %v", err)
	}
	for _, tool := range tools.Tools {
		switch tool.Name {
		case "create_monitors", "create_group", "rename_group", "set_monitor_paused":
			t.Errorf("write tool %q advertised to a read-only caller", tool.Name)
		}
	}

	// And calling it by name anyway gets nowhere.
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "create_group",
		Arguments: map[string]any{"name": "Sneaky"},
	})
	if err == nil && !res.IsError {
		t.Fatal("expected a read-only caller to be refused")
	}
}

func TestEditorSeesWriteTools(t *testing.T) {
	s, _ := newTestServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.URL}, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = session.Close() }()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("tools/list failed: %v", err)
	}
	found := make(map[string]bool, len(tools.Tools))
	for _, tool := range tools.Tools {
		found[tool.Name] = true
	}
	for _, want := range []string{"create_monitors", "create_group", "rename_group", "set_monitor_paused"} {
		if !found[want] {
			t.Errorf("expected write tool %q for an editor", want)
		}
	}
}

// A list of domains usually has one bad entry. Failing the whole batch for it would leave
// the model guessing which ones already exist before it could retry.
func TestCreateMonitorsReportsPerEntry(t *testing.T) {
	s, _ := newTestServer(t)

	_, out, err := s.createMonitors(context.Background(), nil, CreateMonitorsInput{
		Monitors: []NewMonitor{
			{Target: "https://one.example.com"},
			{Target: "https://two.example.com", Group: "Nope"},
			{Target: "https://three.example.com", Name: "Three", Interval: 30},
		},
	})
	if err != nil {
		t.Fatalf("create_monitors failed: %v", err)
	}

	if out.Created != 2 || out.Failed != 1 {
		t.Fatalf("expected 2 created and 1 failed, got %d and %d", out.Created, out.Failed)
	}
	if out.Results[1].Created || !strings.Contains(out.Results[1].Error, "Nope") {
		t.Errorf("expected the bad group to be named in its own result, got %+v", out.Results[1])
	}
	// The name falls back to the url, so a bare list of domains still produces something
	// readable on the dashboard.
	if out.Results[0].Name != "https://one.example.com" {
		t.Errorf("expected the name to default to the url, got %q", out.Results[0].Name)
	}
}

func TestCreateMonitorsRejectsEmptyList(t *testing.T) {
	s, _ := newTestServer(t)
	if _, _, err := s.createMonitors(context.Background(), nil, CreateMonitorsInput{}); err == nil {
		t.Fatal("expected an error when no monitors are passed")
	}
}

func TestResolveGroupFallsBackToDefault(t *testing.T) {
	s, _ := newTestServer(t)

	id, err := s.resolveGroupID("")
	if err != nil {
		t.Fatalf("expected the Default group to be found: %v", err)
	}
	if id != "g-default" {
		t.Errorf("expected g-default, got %q", id)
	}

	if _, err := s.resolveGroupID("Missing"); err == nil {
		t.Fatal("expected an unknown group to be an error")
	} else if !strings.Contains(err.Error(), "create_group") {
		t.Errorf("expected the error to point at create_group, got %q", err)
	}
}

// Events are stored in UTC. Building the window from a local clock shifted it by the
// offset and the tool returned nothing at all, which reads as "this monitor is fine".
func TestGetMonitorEventsWindowUsesUTC(t *testing.T) {
	s, store := newTestServer(t)
	seedMonitor(t, store, "m-1", "Recent", "https://one.example.com")
	if err := store.CreateEvent("m-1", "down", "Monitor is down"); err != nil {
		t.Fatalf("failed to seed event: %v", err)
	}

	_, out, err := s.getMonitorEvents(context.Background(), nil, GetMonitorEventsInput{Monitor: "Recent"})
	if err != nil {
		t.Fatalf("get_monitor_events failed: %v", err)
	}
	if len(out.Events) == 0 {
		t.Fatal("expected the event just written to fall inside the default window")
	}
}

// The body the target returned is usually the fastest route to the cause, so it has to
// survive the trip out to the model.
func TestEventSummaryCarriesTheServerResponse(t *testing.T) {
	body := `{"error":"upstream_unavailable"}`
	headers := `{"Retry-After":"30"}`
	status, latency := 503, int64(42)

	got := summarizeEvents([]db.MonitorEvent{{
		Type:            "down",
		Message:         "Monitor is down (Status: 503)",
		Timestamp:       time.Now(),
		StatusCode:      &status,
		Latency:         &latency,
		ResponseBody:    &body,
		ResponseHeaders: &headers,
	}})

	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	if got[0].ResponseBody != body {
		t.Errorf("expected the response body to be carried through, got %q", got[0].ResponseBody)
	}
	if got[0].ResponseHeaders != headers || got[0].Latency != latency || got[0].Status != status {
		t.Errorf("expected the diagnostics to survive, got %+v", got[0])
	}
}

// An event that is enabled but batched is not immediate. Reporting it in both lists
// would repeat exactly the confusion this tool exists to clear up.
func TestNotificationConfigDoesNotCallBatchedEventsImmediate(t *testing.T) {
	s, store := newTestServer(t)
	for k, v := range map[string]string{
		"notification.digest.enabled":     "true",
		"notification.digest.event_types": "down,flapping",
	} {
		if err := store.SetSetting(k, v); err != nil {
			t.Fatalf("failed to set %s: %v", k, err)
		}
	}

	_, out, err := s.getNotificationConfig(context.Background(), nil, struct{}{})
	if err != nil {
		t.Fatalf("get_notification_config failed: %v", err)
	}

	for _, e := range out.ImmediateEvents {
		if e == "down" || e == "flapping" {
			t.Errorf("%q is batched, so it must not be listed as immediate: %v", e, out.ImmediateEvents)
		}
	}
	if len(out.ImmediateEvents) == 0 {
		t.Error("expected the events that are not batched to still be listed as immediate")
	}
}

// With the digest off, nothing is diverted, so every enabled event is immediate again.
func TestNotificationConfigWithDigestOff(t *testing.T) {
	s, store := newTestServer(t)
	if err := store.SetSetting("notification.digest.event_types", "down"); err != nil {
		t.Fatalf("failed to set digest types: %v", err)
	}

	_, out, err := s.getNotificationConfig(context.Background(), nil, struct{}{})
	if err != nil {
		t.Fatalf("get_notification_config failed: %v", err)
	}
	found := false
	for _, e := range out.ImmediateEvents {
		if e == "down" {
			found = true
		}
	}
	if !found {
		t.Errorf("with the digest disabled, down should be immediate: %v", out.ImmediateEvents)
	}
}

// Monitors carry a check type now, so the tools have to say which check each one runs.
// Without it an assistant looking at "db.internal:5432" cannot tell a TCP monitor from
// an HTTP one with a strange URL.
func TestListMonitorsReportsCheckType(t *testing.T) {
	s, store := newTestServer(t)
	if err := store.CreateMonitor(db.Monitor{
		ID: "m-tcp", Type: db.MonitorTypeTCP, GroupID: "g-default",
		Name: "Postgres", URL: "db.internal:5432", Active: true, Interval: 60,
	}); err != nil {
		t.Fatalf("failed to seed monitor: %v", err)
	}
	seedMonitor(t, store, "m-http", "API", "https://api.example.com")

	_, out, err := s.listMonitors(context.Background(), nil, ListMonitorsInput{})
	if err != nil {
		t.Fatalf("list_monitors failed: %v", err)
	}

	types := map[string]string{}
	for _, m := range out.Monitors {
		types[m.Name] = m.Type
	}
	if types["Postgres"] != db.MonitorTypeTCP {
		t.Errorf("expected the TCP monitor to report its type, got %q", types["Postgres"])
	}
	// A monitor stored before check types existed reads back blank; it is http.
	if types["API"] != db.MonitorTypeHTTP {
		t.Errorf("expected an untyped monitor to report http, got %q", types["API"])
	}
}

func TestGetMonitorReportsCheckType(t *testing.T) {
	s, store := newTestServer(t)
	if err := store.CreateMonitor(db.Monitor{
		ID: "m-ping", Type: db.MonitorTypePing, GroupID: "g-default",
		Name: "Router", URL: "192.168.1.1", Active: true, Interval: 60,
	}); err != nil {
		t.Fatalf("failed to seed monitor: %v", err)
	}

	_, out, err := s.getMonitor(context.Background(), nil, GetMonitorInput{Monitor: "Router"})
	if err != nil {
		t.Fatalf("get_monitor failed: %v", err)
	}
	if out.Type != db.MonitorTypePing {
		t.Errorf("expected ping, got %q", out.Type)
	}
}

// Creating a non-HTTP monitor has to work through the tool, not just through the UI.
func TestCreateMonitorsPassesTheTypeThrough(t *testing.T) {
	s, _ := newTestServer(t)

	_, out, err := s.createMonitors(context.Background(), nil, CreateMonitorsInput{
		Monitors: []NewMonitor{
			{Target: "db.internal:5432", Type: "tcp", Name: "Postgres"},
			{Target: "192.168.1.1", Type: "PING", Name: "Router"},
			{Target: "https://api.example.com", Name: "API"},
		},
	})
	if err != nil {
		t.Fatalf("create_monitors failed: %v", err)
	}
	if out.Created != 3 {
		t.Fatalf("expected 3 created, got %d: %+v", out.Created, out.Results)
	}

	_, listed, err := s.listMonitors(context.Background(), nil, ListMonitorsInput{})
	if err != nil {
		t.Fatalf("list_monitors failed: %v", err)
	}
	got := map[string]string{}
	for _, m := range listed.Monitors {
		got[m.Name] = m.Type
	}
	if got["Postgres"] != db.MonitorTypeTCP {
		t.Errorf("expected tcp, got %q", got["Postgres"])
	}
	// Case should not matter: a model writing "PING" means ping.
	if got["Router"] != db.MonitorTypePing {
		t.Errorf("expected ping from an uppercase type, got %q", got["Router"])
	}
	if got["API"] != db.MonitorTypeHTTP {
		t.Errorf("expected an omitted type to default to http, got %q", got["API"])
	}
}
