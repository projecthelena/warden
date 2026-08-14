package mcp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// checkNowWait is how long a forced check gets to come back before we admit we do not
// have an answer yet. Checks time out at 5s by default, plus a little slack.
const checkNowWait = 8 * time.Second

func (s *Server) addDiagnosticTools(srv *mcp.Server, readOnly *mcp.ToolAnnotations) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:  "get_monitor_latency",
		Title: "Get a monitor's response times over time",
		Description: "Latency samples for a monitor, with the slowest and average. Use this to tell a " +
			"target that fell over suddenly from one that had been degrading for a while.",
		Annotations: readOnly,
	}, s.getMonitorLatency)

	mcp.AddTool(srv, &mcp.Tool{
		Name:  "get_notification_config",
		Title: "Get the notification settings",
		Description: "How alerting is configured: confirmation threshold, cooldown, flap detection, " +
			"which events notify, what the daily digest swallows, and which channels exist. Use this " +
			"to explain why an outage did or did not produce an alert. Webhook URLs are not included.",
		Annotations: readOnly,
	}, s.getNotificationConfig)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_ssl_warnings",
		Title:       "List certificates about to expire",
		Description: "Monitors whose TLS certificate is close to expiring, across all groups.",
		Annotations: readOnly,
	}, s.listSSLWarnings)

	mcp.AddTool(srv, &mcp.Tool{
		Name:  "check_now",
		Title: "Run a monitor's check immediately",
		Description: "Forces a check right now instead of waiting for the next interval, and returns " +
			"the result. Use it to confirm a fix actually worked.",
	}, s.checkNow)
}

type GetMonitorLatencyInput struct {
	Monitor string `json:"monitor" jsonschema:"the monitor's name or id"`
	Hours   int    `json:"hours,omitempty" jsonschema:"how far back to look in hours (default 24)"`
}

type LatencySample struct {
	Timestamp string `json:"timestamp"`
	Latency   int64  `json:"latencyMs"`
	Failed    bool   `json:"failed,omitempty"`
}

type GetMonitorLatencyOutput struct {
	Monitor   string          `json:"monitor"`
	Hours     int             `json:"hours"`
	Samples   []LatencySample `json:"samples"`
	AverageMs int64           `json:"averageMs"`
	SlowestMs int64           `json:"slowestMs"`
	Failures  int             `json:"failures"`
}

func (s *Server) getMonitorLatency(ctx context.Context, _ *mcp.CallToolRequest, in GetMonitorLatencyInput) (*mcp.CallToolResult, GetMonitorLatencyOutput, error) {
	m, err := s.resolveMonitor(in.Monitor)
	if err != nil {
		return nil, GetMonitorLatencyOutput{}, err
	}
	hours := in.Hours
	if hours <= 0 {
		hours = 24
	}

	points, err := s.store.GetLatencyStats(m.ID, hours)
	if err != nil {
		return nil, GetMonitorLatencyOutput{}, fmt.Errorf("failed to load latency: %w", err)
	}

	out := GetMonitorLatencyOutput{Monitor: m.Name, Hours: hours, Samples: make([]LatencySample, 0, len(points))}
	var total int64
	var counted int64
	for _, p := range points {
		out.Samples = append(out.Samples, LatencySample{
			Timestamp: p.Timestamp.UTC().Format(time.RFC3339),
			Latency:   p.Latency,
			Failed:    p.Failed,
		})
		if p.Failed {
			out.Failures++
			continue
		}
		// A failed check's latency is how long it took to fail, which would drag the
		// average somewhere meaningless.
		total += p.Latency
		counted++
		if p.Latency > out.SlowestMs {
			out.SlowestMs = p.Latency
		}
	}
	if counted > 0 {
		out.AverageMs = total / counted
	}

	return nil, out, nil
}

type ChannelSummary struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

type GetNotificationConfigOutput struct {
	ConfirmationThreshold int              `json:"confirmationThreshold"`
	CooldownMinutes       int              `json:"cooldownMinutes"`
	FlapDetectionEnabled  bool             `json:"flapDetectionEnabled"`
	ImmediateEvents       []string         `json:"immediateEvents"`
	DigestEnabled         bool             `json:"digestEnabled"`
	DigestTime            string           `json:"digestTime,omitempty"`
	BatchedEvents         []string         `json:"batchedEvents"`
	Channels              []ChannelSummary `json:"channels"`
	Note                  string           `json:"note"`
}

// getNotificationConfig answers "why did I not get told". The batched events are the
// part people miss: an event listed there is sent in the daily digest *instead of*
// immediately, so putting "down" in it silences outage alerts.
func (s *Server) getNotificationConfig(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, GetNotificationConfigOutput, error) {
	out := GetNotificationConfigOutput{
		ConfirmationThreshold: s.settingInt("notification.confirmation_threshold", 3),
		CooldownMinutes:       s.settingInt("notification.cooldown_minutes", 30),
		FlapDetectionEnabled:  s.settingBool("notification.flap_detection_enabled", true),
		DigestEnabled:         s.settingBool("notification.digest.enabled", false),
		DigestTime:            s.setting("notification.digest.time"),
		BatchedEvents:         splitList(s.setting("notification.digest.event_types")),
		Note: "An event listed in batchedEvents is sent in the daily digest instead of immediately, " +
			"not as well as. Flapping monitors also have their down alerts suppressed until they settle.",
	}

	// An event is only immediate if it is enabled *and* not being diverted to the digest.
	// Reporting it as both would repeat the confusion this tool exists to clear up.
	batched := make(map[string]bool, len(out.BatchedEvents))
	if out.DigestEnabled {
		for _, e := range out.BatchedEvents {
			batched[e] = true
		}
	}
	out.ImmediateEvents = []string{}
	for _, event := range []string{"down", "up", "degraded", "flapping", "stabilized", "ssl_expiring"} {
		if s.settingBool("notification.event."+event+".enabled", true) && !batched[event] {
			out.ImmediateEvents = append(out.ImmediateEvents, event)
		}
	}

	channels, err := s.store.GetNotificationChannels()
	if err != nil {
		return nil, GetNotificationConfigOutput{}, fmt.Errorf("failed to load channels: %w", err)
	}
	out.Channels = make([]ChannelSummary, 0, len(channels))
	for _, c := range channels {
		// Deliberately no config: that is where the webhook URL lives.
		out.Channels = append(out.Channels, ChannelSummary{Name: c.Name, Type: c.Type, Enabled: c.Enabled})
	}

	return nil, out, nil
}

type SSLWarning struct {
	Monitor string `json:"monitor"`
	Group   string `json:"group"`
	Message string `json:"message"`
	Seen    string `json:"seenAt"`
}

type ListSSLWarningsOutput struct {
	Warnings []SSLWarning `json:"warnings"`
}

func (s *Server) listSSLWarnings(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ListSSLWarningsOutput, error) {
	warnings, err := s.store.GetActiveSSLWarnings()
	if err != nil {
		return nil, ListSSLWarningsOutput{}, fmt.Errorf("failed to load SSL warnings: %w", err)
	}

	out := ListSSLWarningsOutput{Warnings: make([]SSLWarning, 0, len(warnings))}
	for _, w := range warnings {
		out.Warnings = append(out.Warnings, SSLWarning{
			Monitor: w.MonitorName,
			Group:   w.GroupName,
			Message: w.Message,
			Seen:    w.Timestamp.UTC().Format(time.RFC3339),
		})
	}
	return nil, out, nil
}

type CheckNowInput struct {
	Monitor string `json:"monitor" jsonschema:"the monitor's name or id"`
}

type CheckNowOutput struct {
	Monitor   string `json:"monitor"`
	Status    string `json:"status"`
	LatencyMs int64  `json:"latencyMs"`
	Checked   bool   `json:"checked"`
	Note      string `json:"note,omitempty"`
}

func (s *Server) checkNow(ctx context.Context, _ *mcp.CallToolRequest, in CheckNowInput) (*mcp.CallToolResult, CheckNowOutput, error) {
	m, err := s.resolveMonitor(in.Monitor)
	if err != nil {
		return nil, CheckNowOutput{}, err
	}
	if !m.Active {
		return nil, CheckNowOutput{Monitor: m.Name, Status: "paused", Note: "the monitor is paused, resume it before checking"}, nil
	}

	checked := s.manager.CheckNow(m.ID, checkNowWait)
	status, latency := s.liveStatus(*m)

	out := CheckNowOutput{Monitor: m.Name, Status: status, LatencyMs: latency, Checked: checked}
	if !checked {
		out.Note = "no fresh result within the timeout; the status shown is the last known one"
	}
	return nil, out, nil
}

func (s *Server) setting(key string) string {
	v, err := s.store.GetSetting(key)
	if err != nil {
		return ""
	}
	return v
}

func (s *Server) settingInt(key string, fallback int) int {
	if v, err := strconv.Atoi(s.setting(key)); err == nil {
		return v
	}
	return fallback
}

func (s *Server) settingBool(key string, fallback bool) bool {
	switch s.setting(key) {
	case "true":
		return true
	case "false":
		return false
	}
	return fallback
}

func splitList(v string) []string {
	if strings.TrimSpace(v) == "" {
		return []string{}
	}
	parts := strings.Split(v, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
