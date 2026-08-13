// Package mcp exposes Warden's monitoring data over the Model Context Protocol, so an
// assistant can answer questions about it without being handed the whole REST API.
//
// The tools are shaped like the questions people ask ("what is down", "what happened to
// this monitor") rather than like the endpoints underneath. What a caller gets follows
// the API key's role: a viewer key sees only the tools that read, an editor key also
// gets the ones that create and change things.
package mcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/uptime"
)

// maxEvents caps how much history a single call can pull back, so a broad question
// cannot drag the whole events table through the context window.
const maxEvents = 200

// Server wires Warden's store and live monitor state into MCP tools.
type Server struct {
	store   *db.Store
	manager *uptime.Manager
	version string

	// writer performs the changes. Nil means read-only.
	writer Writer
	// canWrite reports whether the caller behind a request may change anything, so the
	// tool set follows the API key's role rather than being the same for everyone.
	canWrite func(*http.Request) bool
}

func NewServer(store *db.Store, manager *uptime.Manager, version string, writer Writer, canWrite func(*http.Request) bool) *Server {
	return &Server{store: store, manager: manager, version: version, writer: writer, canWrite: canWrite}
}

// Handler returns the HTTP handler for the MCP endpoint. Mount it behind the same auth
// as the rest of the API.
func (s *Server) Handler() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return s.newMCPServer(s.writer != nil && s.canWrite != nil && s.canWrite(r))
	}, nil)
}

func (s *Server) newMCPServer(writable bool) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "warden",
		Title:   "Warden uptime monitoring",
		Version: s.version,
	}, nil)

	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_monitors",
		Title:       "List monitors and their current status",
		Description: "Every monitor with its current status, check type, latency and group. Use status to narrow it down, for example status=down to answer what is broken right now.",
		Annotations: readOnly,
	}, s.listMonitors)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_monitor",
		Title:       "Get one monitor in detail",
		Description: "Current status, uptime over 24h/7d/30d and the most recent events for a single monitor. Accepts the monitor's name or its id.",
		Annotations: readOnly,
	}, s.getMonitor)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_incidents",
		Title:       "List outages",
		Description: "Outages across all monitors, open ones first. Use this to answer what happened over a period, or whether several monitors failed together.",
		Annotations: readOnly,
	}, s.listIncidents)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_monitor_events",
		Title:       "Get a monitor's raw events",
		Description: "The individual check events for a monitor over a time window, including the error each failed check returned. Use this to work out why a monitor failed.",
		Annotations: readOnly,
	}, s.getMonitorEvents)

	s.addDiagnosticTools(srv, readOnly)

	// A viewer key never sees the write tools at all, so the model is not left offering
	// the user something the server will refuse.
	if writable {
		s.addWriteTools(srv)
	}

	return srv
}

type ListMonitorsInput struct {
	Status string `json:"status,omitempty" jsonschema:"filter by current status: up, down, degraded or paused"`
}

type MonitorSummary struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Target  string `json:"target"`
	Group   string `json:"group"`
	Status  string `json:"status"`
	Latency int64  `json:"latencyMs"`
}

type ListMonitorsOutput struct {
	Monitors []MonitorSummary `json:"monitors"`
	Down     int              `json:"downCount"`
	Total    int              `json:"totalCount"`
}

func (s *Server) listMonitors(ctx context.Context, _ *mcp.CallToolRequest, in ListMonitorsInput) (*mcp.CallToolResult, ListMonitorsOutput, error) {
	monitors, err := s.store.GetMonitors()
	if err != nil {
		return nil, ListMonitorsOutput{}, fmt.Errorf("failed to load monitors: %w", err)
	}
	groups, err := s.groupNames()
	if err != nil {
		return nil, ListMonitorsOutput{}, err
	}

	want := strings.ToLower(strings.TrimSpace(in.Status))
	out := ListMonitorsOutput{Monitors: []MonitorSummary{}, Total: len(monitors)}

	for _, m := range monitors {
		status, latency := s.liveStatus(m)
		if status == "down" {
			out.Down++
		}
		if want != "" && want != status {
			continue
		}
		out.Monitors = append(out.Monitors, MonitorSummary{
			ID:      m.ID,
			Name:    m.Name,
			Type:    db.NormalizeMonitorType(m.Type),
			Target:  m.URL,
			Group:   groups[m.GroupID],
			Status:  status,
			Latency: latency,
		})
	}

	return nil, out, nil
}

type GetMonitorInput struct {
	Monitor string `json:"monitor" jsonschema:"the monitor's name or id"`
}

type EventSummary struct {
	Type      string `json:"type"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Error     string `json:"error,omitempty"`
	Status    int    `json:"statusCode,omitempty"`
	Latency   int64  `json:"latencyMs,omitempty"`
	// What the target actually returned on a failed check. Usually the fastest route to
	// the cause, and already filtered of sensitive headers when it was stored.
	ResponseBody    string `json:"responseBody,omitempty"`
	ResponseHeaders string `json:"responseHeaders,omitempty"`
}

type GetMonitorOutput struct {
	MonitorSummary
	Interval     int            `json:"intervalSeconds"`
	Uptime24h    float64        `json:"uptime24h"`
	Uptime7d     float64        `json:"uptime7d"`
	Uptime30d    float64        `json:"uptime30d"`
	RecentEvents []EventSummary `json:"recentEvents"`
}

func (s *Server) getMonitor(ctx context.Context, _ *mcp.CallToolRequest, in GetMonitorInput) (*mcp.CallToolResult, GetMonitorOutput, error) {
	m, err := s.resolveMonitor(in.Monitor)
	if err != nil {
		return nil, GetMonitorOutput{}, err
	}
	groups, err := s.groupNames()
	if err != nil {
		return nil, GetMonitorOutput{}, err
	}

	status, latency := s.liveStatus(*m)
	day, week, month, err := s.store.GetUptimeStats(m.ID)
	if err != nil {
		return nil, GetMonitorOutput{}, fmt.Errorf("failed to load uptime stats: %w", err)
	}

	events, err := s.store.GetMonitorEvents(m.ID, 15)
	if err != nil {
		return nil, GetMonitorOutput{}, fmt.Errorf("failed to load events: %w", err)
	}

	return nil, GetMonitorOutput{
		MonitorSummary: MonitorSummary{
			ID:      m.ID,
			Name:    m.Name,
			Type:    db.NormalizeMonitorType(m.Type),
			Target:  m.URL,
			Group:   groups[m.GroupID],
			Status:  status,
			Latency: latency,
		},
		Interval:     m.Interval,
		Uptime24h:    day,
		Uptime7d:     week,
		Uptime30d:    month,
		RecentEvents: summarizeEvents(events),
	}, nil
}

type ListIncidentsInput struct {
	SinceHours int `json:"sinceHours,omitempty" jsonschema:"how far back to look in hours (default 24)"`
}

type IncidentSummary struct {
	Monitor  string `json:"monitor"`
	Group    string `json:"group"`
	Type     string `json:"type"`
	Summary  string `json:"summary"`
	Started  string `json:"startedAt"`
	Ended    string `json:"endedAt,omitempty"`
	Ongoing  bool   `json:"ongoing"`
	Duration string `json:"duration"`
}

type ListIncidentsOutput struct {
	Incidents []IncidentSummary `json:"incidents"`
	Ongoing   int               `json:"ongoingCount"`
}

func (s *Server) listIncidents(ctx context.Context, _ *mcp.CallToolRequest, in ListIncidentsInput) (*mcp.CallToolResult, ListIncidentsOutput, error) {
	hours := in.SinceHours
	if hours <= 0 {
		hours = 24
	}
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)

	active, err := s.store.GetActiveOutages()
	if err != nil {
		return nil, ListIncidentsOutput{}, fmt.Errorf("failed to load active outages: %w", err)
	}
	resolved, err := s.store.GetResolvedOutages(since)
	if err != nil {
		return nil, ListIncidentsOutput{}, fmt.Errorf("failed to load resolved outages: %w", err)
	}

	out := ListIncidentsOutput{Incidents: []IncidentSummary{}, Ongoing: len(active)}
	for _, o := range append(active, resolved...) {
		out.Incidents = append(out.Incidents, incidentSummary(o))
	}

	return nil, out, nil
}

type GetMonitorEventsInput struct {
	Monitor    string `json:"monitor" jsonschema:"the monitor's name or id"`
	SinceHours int    `json:"sinceHours,omitempty" jsonschema:"how far back to look in hours (default 24)"`
	Limit      int    `json:"limit,omitempty" jsonschema:"maximum events to return (default 50, max 200)"`
}

type GetMonitorEventsOutput struct {
	Monitor string         `json:"monitor"`
	Events  []EventSummary `json:"events"`
}

func (s *Server) getMonitorEvents(ctx context.Context, _ *mcp.CallToolRequest, in GetMonitorEventsInput) (*mcp.CallToolResult, GetMonitorEventsOutput, error) {
	m, err := s.resolveMonitor(in.Monitor)
	if err != nil {
		return nil, GetMonitorEventsOutput{}, err
	}

	hours := in.SinceHours
	if hours <= 0 {
		hours = 24
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > maxEvents {
		limit = maxEvents
	}

	// Timestamps are stored in UTC. Building this window from a local clock shifts it
	// by the offset, which shows up as a monitor that mysteriously has no events.
	end := time.Now().UTC()
	events, err := s.store.GetMonitorEventsBetween(m.ID, end.Add(-time.Duration(hours)*time.Hour), end, limit)
	if err != nil {
		return nil, GetMonitorEventsOutput{}, fmt.Errorf("failed to load events: %w", err)
	}

	return nil, GetMonitorEventsOutput{Monitor: m.Name, Events: summarizeEvents(events)}, nil
}

// resolveMonitor accepts a name or an id, because an assistant asked about "Google" has
// no way to know the id is m-google-a1b2c3.
func (s *Server) resolveMonitor(nameOrID string) (*db.Monitor, error) {
	query := strings.TrimSpace(nameOrID)
	if query == "" {
		return nil, fmt.Errorf("monitor is required: pass a monitor name or id")
	}

	monitors, err := s.store.GetMonitors()
	if err != nil {
		return nil, fmt.Errorf("failed to load monitors: %w", err)
	}
	for _, m := range monitors {
		if m.ID == query || strings.EqualFold(m.Name, query) {
			return &m, nil
		}
	}

	names := make([]string, 0, len(monitors))
	for _, m := range monitors {
		names = append(names, m.Name)
	}
	return nil, fmt.Errorf("no monitor named %q. Known monitors: %s", query, strings.Join(names, ", "))
}

// liveStatus reads the status the manager is holding, which is fresher than anything in
// the database and is what the dashboard shows.
func (s *Server) liveStatus(m db.Monitor) (string, int64) {
	if !m.Active {
		return "paused", 0
	}
	mon := s.manager.GetMonitor(m.ID)
	if mon == nil {
		return "unknown", 0
	}
	isUp, latency, hasHistory, degraded := mon.GetLastStatus()
	switch {
	case !hasHistory:
		return "unknown", 0
	case !isUp:
		return "down", latency
	case degraded:
		return "degraded", latency
	default:
		return "up", latency
	}
}

func (s *Server) groupNames() (map[string]string, error) {
	groups, err := s.store.GetGroups()
	if err != nil {
		return nil, fmt.Errorf("failed to load groups: %w", err)
	}
	names := make(map[string]string, len(groups))
	for _, g := range groups {
		names[g.ID] = g.Name
	}
	return names, nil
}

func summarizeEvents(events []db.MonitorEvent) []EventSummary {
	out := make([]EventSummary, 0, len(events))
	for _, e := range events {
		summary := EventSummary{
			Type:      e.Type,
			Message:   e.Message,
			Timestamp: e.Timestamp.UTC().Format(time.RFC3339),
		}
		if e.ErrorMessage != nil {
			summary.Error = *e.ErrorMessage
		}
		if e.StatusCode != nil {
			summary.Status = *e.StatusCode
		}
		if e.Latency != nil {
			summary.Latency = *e.Latency
		}
		if e.ResponseBody != nil {
			summary.ResponseBody = *e.ResponseBody
		}
		if e.ResponseHeaders != nil {
			summary.ResponseHeaders = *e.ResponseHeaders
		}
		out = append(out, summary)
	}
	return out
}

func incidentSummary(o db.MonitorOutage) IncidentSummary {
	end := time.Now()
	summary := IncidentSummary{
		Monitor: o.MonitorName,
		Group:   o.GroupName,
		Type:    o.Type,
		Summary: o.Summary,
		Started: o.StartTime.UTC().Format(time.RFC3339),
		Ongoing: o.EndTime == nil,
	}
	if o.EndTime != nil {
		end = *o.EndTime
		summary.Ended = o.EndTime.UTC().Format(time.RFC3339)
	}
	summary.Duration = end.Sub(o.StartTime).Round(time.Second).String()
	return summary
}
