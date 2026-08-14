package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/projecthelena/warden/internal/db"
)

// Writer is the creation logic the API handlers already use, handed to the MCP tools so
// the two surfaces validate identically instead of drifting apart.
type Writer interface {
	AddMonitor(in MonitorInput) (db.Monitor, error)
	AddGroup(name string) (db.Group, error)
	SetMonitorActive(id string, active bool) error
	RenameGroup(id, name string) error
}

// MonitorInput mirrors the API's creation input. It is redeclared here so this package
// does not have to import the API package it is mounted by.
type MonitorInput struct {
	Name     string
	Type     string
	URL      string
	GroupID  string
	Interval int
}

const (
	// defaultInterval matches what the dashboard offers when creating a monitor.
	defaultInterval = 60

	// maxBatch bounds one call. Comfortably more than anyone pastes by hand, and it
	// stops a single instruction, including one arriving inside a monitored target's
	// error body, from filling the install with monitors.
	maxBatch = 100
)

func (s *Server) addWriteTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:  "create_monitors",
		Title: "Create one or more monitors",
		Description: "Create monitors from a list of targets. Pass the whole list in one call: " +
			"each entry succeeds or fails on its own and comes back with its own result. " +
			"Only url is required; type defaults to http, name to the target, group to " +
			"Default and interval to 60s.",
	}, s.createMonitors)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_group",
		Title:       "Create a group",
		Description: "Create a group to organise monitors into. Returns the group id to pass to create_monitors.",
	}, s.createGroup)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "rename_group",
		Title:       "Rename a group",
		Description: "Rename an existing group. Accepts the group's current name or its id.",
	}, s.renameGroup)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_monitor_paused",
		Title:       "Pause or resume a monitor",
		Description: "Stop or restart checking a monitor. Pausing keeps the monitor and its history, it just stops running checks.",
	}, s.setMonitorPaused)
}

type NewMonitor struct {
	Target   string `json:"url" jsonschema:"what to check: an http(s) URL, host:port for tcp, or a hostname for ping and dns"`
	Type     string `json:"type,omitempty" jsonschema:"check type: http (default), tcp, ping or dns"`
	Name     string `json:"name,omitempty" jsonschema:"display name (defaults to the target)"`
	Group    string `json:"group,omitempty" jsonschema:"group name or id (defaults to the Default group)"`
	Interval int    `json:"intervalSeconds,omitempty" jsonschema:"seconds between checks, at least 10 (default 60)"`
}

type CreateMonitorsInput struct {
	Monitors []NewMonitor `json:"monitors" jsonschema:"the monitors to create"`
}

type CreateResult struct {
	URL     string `json:"url"`
	Name    string `json:"name,omitempty"`
	ID      string `json:"id,omitempty"`
	Created bool   `json:"created"`
	Error   string `json:"error,omitempty"`
}

type CreateMonitorsOutput struct {
	Results []CreateResult `json:"results"`
	Created int            `json:"createdCount"`
	Failed  int            `json:"failedCount"`
}

// createMonitors keeps going after a failure and reports per entry. A list of domains
// usually has one typo in it, and failing the whole batch for that would mean the model
// has to work out which ones already exist before retrying.
func (s *Server) createMonitors(ctx context.Context, _ *mcp.CallToolRequest, in CreateMonitorsInput) (*mcp.CallToolResult, CreateMonitorsOutput, error) {
	if len(in.Monitors) == 0 {
		return nil, CreateMonitorsOutput{}, fmt.Errorf("monitors is required: pass at least one entry")
	}
	if len(in.Monitors) > maxBatch {
		return nil, CreateMonitorsOutput{}, fmt.Errorf("too many monitors in one call: %d, maximum is %d. Split the list", len(in.Monitors), maxBatch)
	}

	out := CreateMonitorsOutput{Results: make([]CreateResult, 0, len(in.Monitors))}
	for _, entry := range in.Monitors {
		result := CreateResult{URL: strings.TrimSpace(entry.Target), Name: entry.Name}

		groupID, err := s.resolveGroupID(entry.Group)
		if err != nil {
			result.Error = err.Error()
			out.Results = append(out.Results, result)
			out.Failed++
			continue
		}

		name := strings.TrimSpace(entry.Name)
		if name == "" {
			name = result.URL
		}
		interval := entry.Interval
		if interval <= 0 {
			interval = defaultInterval
		}

		m, err := s.writer.AddMonitor(MonitorInput{
			Name:     name,
			Type:     strings.ToLower(strings.TrimSpace(entry.Type)),
			URL:      result.URL,
			GroupID:  groupID,
			Interval: interval,
		})
		if err != nil {
			result.Error = err.Error()
			out.Failed++
		} else {
			result.ID, result.Name, result.Created = m.ID, m.Name, true
			out.Created++
		}
		out.Results = append(out.Results, result)
	}

	return nil, out, nil
}

type CreateGroupInput struct {
	Name string `json:"name" jsonschema:"the group's display name"`
}

type GroupOutput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Server) createGroup(ctx context.Context, _ *mcp.CallToolRequest, in CreateGroupInput) (*mcp.CallToolResult, GroupOutput, error) {
	g, err := s.writer.AddGroup(strings.TrimSpace(in.Name))
	if err != nil {
		return nil, GroupOutput{}, err
	}
	return nil, GroupOutput{ID: g.ID, Name: g.Name}, nil
}

type RenameGroupInput struct {
	Group string `json:"group" jsonschema:"the group's current name or id"`
	Name  string `json:"name" jsonschema:"the new name"`
}

func (s *Server) renameGroup(ctx context.Context, _ *mcp.CallToolRequest, in RenameGroupInput) (*mcp.CallToolResult, GroupOutput, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, GroupOutput{}, fmt.Errorf("name is required")
	}
	id, err := s.resolveGroupID(in.Group)
	if err != nil {
		return nil, GroupOutput{}, err
	}
	if err := s.writer.RenameGroup(id, name); err != nil {
		return nil, GroupOutput{}, err
	}
	return nil, GroupOutput{ID: id, Name: name}, nil
}

type SetMonitorPausedInput struct {
	Monitor string `json:"monitor" jsonschema:"the monitor's name or id"`
	Paused  bool   `json:"paused" jsonschema:"true to stop checking it, false to resume"`
}

type SetMonitorPausedOutput struct {
	Monitor string `json:"monitor"`
	Paused  bool   `json:"paused"`
}

func (s *Server) setMonitorPaused(ctx context.Context, _ *mcp.CallToolRequest, in SetMonitorPausedInput) (*mcp.CallToolResult, SetMonitorPausedOutput, error) {
	m, err := s.resolveMonitor(in.Monitor)
	if err != nil {
		return nil, SetMonitorPausedOutput{}, err
	}
	if err := s.writer.SetMonitorActive(m.ID, !in.Paused); err != nil {
		return nil, SetMonitorPausedOutput{}, err
	}
	return nil, SetMonitorPausedOutput{Monitor: m.Name, Paused: in.Paused}, nil
}

// resolveGroupID accepts a group name or id, and falls back to the Default group so a
// caller handed nothing but a list of URLs still succeeds.
func (s *Server) resolveGroupID(nameOrID string) (string, error) {
	query := strings.TrimSpace(nameOrID)
	if query == "" {
		query = "Default"
	}

	groups, err := s.store.GetGroups()
	if err != nil {
		return "", fmt.Errorf("failed to load groups: %w", err)
	}
	for _, g := range groups {
		if g.ID == query || strings.EqualFold(g.Name, query) {
			return g.ID, nil
		}
	}

	names := make([]string, 0, len(groups))
	for _, g := range groups {
		names = append(names, g.Name)
	}
	return "", fmt.Errorf("no group named %q. Existing groups: %s. Use create_group to add one", query, strings.Join(names, ", "))
}
