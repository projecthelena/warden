package mcp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/projecthelena/warden/internal/db"
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
		Name:  "list_insights",
		Title: "List detected patterns",
		Description: "Patterns Warden has found in monitor history: latency that climbs and then " +
			"resets, trouble that clusters at one time of day, monitors that always fail together, " +
			"and week-over-week slowdowns that never crossed a threshold. Use this to answer what " +
			"is quietly wrong rather than what is broken right now. Recomputed daily over the last " +
			"14 days; an empty result means nothing stood out.",
		Annotations: readOnly,
	}, s.listInsights)

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
	// What this monitor normally does, from its own recent history. Without it a raw
	// number of milliseconds cannot be judged: 650ms is fine for one target and a
	// two-and-a-half-times regression for another.
	BaselineP50Ms   int64  `json:"baselineP50Ms,omitempty"`
	BaselineP95Ms   int64  `json:"baselineP95Ms,omitempty"`
	BaselineSamples int    `json:"baselineSamples,omitempty"`
	DegradedAboveMs int64  `json:"degradedAboveMs,omitempty"`
	BaselineNote    string `json:"baselineNote,omitempty"`
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

	if b, ok, err := s.store.GetLatencyBaseline(m.ID); err == nil && ok {
		out.BaselineP50Ms = b.P50
		out.BaselineP95Ms = b.P95
		out.BaselineSamples = b.Samples
		out.DegradedAboveMs = degradedAbove(b, s.settingInt("notification.latency.factor_percent", 150),
			int64(s.settingInt("notification.latency.floor_ms", 100)))
		out.BaselineNote = "p50/p95 over this monitor's own recent successful checks; degradedAboveMs is the line it must cross to count as slow."
	}

	return nil, out, nil
}

// degradedAbove mirrors the manager's adaptive threshold so the tool reports the same line
// the alerting layer actually uses.
func degradedAbove(b db.LatencyBaseline, factorPercent int, floorMs int64) int64 {
	byFactor := b.P95 * int64(factorPercent) / 100
	byFloor := b.P95 + floorMs
	if byFactor > byFloor {
		return byFactor
	}
	return byFloor
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
	AlertAfterSeconds     int              `json:"alertAfterSeconds"`
	ReminderMinutes       int              `json:"reminderMinutes"`
	RepeatReminderMinutes int              `json:"repeatReminderMinutes"`
	CorrelationWindowSec  int              `json:"correlationWindowSeconds"`
	CorrelationMinMonitor int              `json:"correlationMinMonitors"`
	CorrelationGroupPct   int              `json:"correlationGroupPercent"`
	ChronicLimit          int              `json:"chronicAlertLimit"`
	ChronicWindowMinutes  int              `json:"chronicWindowMinutes"`
	MutedMonitors         []string         `json:"mutedMonitors"`
	ImmediateEvents       []string         `json:"immediateEvents"`
	DigestEnabled         bool             `json:"digestEnabled"`
	DigestTime            string           `json:"digestTime,omitempty"`
	DigestEvents          []string         `json:"digestEvents"`
	Channels              []ChannelSummary `json:"channels"`
	Note                  string           `json:"note"`
}

// getNotificationConfig answers "why did I not get told". The part people miss now is the
// silent window: a down alert is held back until the monitor has been down for
// alertAfterSeconds, so a short outage is real, recorded, and deliberately unannounced.
func (s *Server) getNotificationConfig(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, GetNotificationConfigOutput, error) {
	out := GetNotificationConfigOutput{
		ConfirmationThreshold: s.settingInt("notification.confirmation_threshold", 3),
		CooldownMinutes:       s.settingInt("notification.cooldown_minutes", 30),
		FlapDetectionEnabled:  s.settingBool("notification.flap_detection_enabled", true),
		AlertAfterSeconds:     s.settingInt("notification.alert.sustained_seconds", 180),
		ReminderMinutes:       s.settingInt("notification.alert.reminder_minutes", 30),
		RepeatReminderMinutes: s.settingInt("notification.alert.repeat_reminder_minutes", 60),
		CorrelationWindowSec:  s.settingInt("notification.correlation.window_seconds", 300),
		CorrelationMinMonitor: s.settingInt("notification.correlation.min_monitors", 3),
		CorrelationGroupPct:   s.settingInt("notification.correlation.group_percent", 30),
		ChronicLimit:          s.settingInt("notification.chronic.limit", 3),
		ChronicWindowMinutes:  s.settingInt("notification.chronic.window_minutes", 1440),
		DigestEnabled:         s.settingBool("notification.digest.enabled", false),
		DigestTime:            s.setting("notification.digest.time"),
		DigestEvents:          splitList(s.setting("notification.digest.event_types")),
		Note: "digestEvents lists what the daily summary contains; it no longer suppresses the " +
			"immediate alert, those are separate decisions now. down and degraded are announced only " +
			"after the monitor has been in that state for alertAfterSeconds, then repeated after " +
			"reminderMinutes and every repeatReminderMinutes while it lasts. A recovery is announced " +
			"only if the outage itself was. Monitors that fail together within correlationWindowSeconds " +
			"are announced as one incident, and a monitor that has already alerted chronicAlertLimit " +
			"times inside chronicWindowMinutes is collapsed into one 'unstable' notice and then goes " +
			"quiet. mutedMonitors never alert at all. Flapping monitors stay suppressed until they settle.",
	}

	monitors, err := s.store.GetMonitors()
	if err != nil {
		return nil, GetNotificationConfigOutput{}, fmt.Errorf("failed to load monitors: %w", err)
	}
	out.MutedMonitors = []string{}
	for _, mon := range monitors {
		if mon.AlertsMuted {
			out.MutedMonitors = append(out.MutedMonitors, mon.Name)
		}
	}

	// Appearing in the digest no longer diverts an event, so the immediate list is just the
	// per-event toggles.
	out.ImmediateEvents = []string{}
	for _, event := range []string{"down", "up", "degraded", "flapping", "stabilized", "ssl_expiring"} {
		if s.settingBool("notification.event."+event+".enabled", true) {
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

type ListInsightsInput struct {
	Monitor string `json:"monitor,omitempty" jsonschema:"limit to one monitor, by name or id"`
	Kind    string `json:"kind,omitempty" jsonschema:"limit to one pattern: latency_sawtooth, periodic_reset, time_of_day, co_failure or latency_drift"`
}

type InsightSummary struct {
	Monitor    string         `json:"monitor"`
	Kind       string         `json:"kind"`
	Summary    string         `json:"summary"`
	Detail     map[string]any `json:"detail,omitempty"`
	Confidence string         `json:"confidence"`
	DetectedAt string         `json:"detectedAt"`
}

type ListInsightsOutput struct {
	Insights []InsightSummary `json:"insights"`
	Count    int              `json:"count"`
	Note     string           `json:"note"`
}

func (s *Server) listInsights(ctx context.Context, _ *mcp.CallToolRequest, in ListInsightsInput) (*mcp.CallToolResult, ListInsightsOutput, error) {
	monitorID := ""
	if in.Monitor != "" {
		m, err := s.resolveMonitor(in.Monitor)
		if err != nil {
			return nil, ListInsightsOutput{}, err
		}
		monitorID = m.ID
	}

	findings, err := s.store.GetMonitorInsights(monitorID)
	if err != nil {
		return nil, ListInsightsOutput{}, fmt.Errorf("failed to load insights: %w", err)
	}

	out := ListInsightsOutput{
		Insights: make([]InsightSummary, 0, len(findings)),
		Note: "Patterns, not incidents: these describe the shape of how a monitor misbehaves " +
			"over two weeks. Warden reports the shape and its numbers; what causes it is still " +
			"a human's call.",
	}
	for _, f := range findings {
		if in.Kind != "" && f.Kind != in.Kind {
			continue
		}
		out.Insights = append(out.Insights, InsightSummary{
			Monitor:    f.MonitorName,
			Kind:       f.Kind,
			Summary:    f.Summary,
			Detail:     f.Detail,
			Confidence: f.Confidence,
			DetectedAt: f.DetectedAt.UTC().Format(time.RFC3339),
		})
	}
	out.Count = len(out.Insights)

	return nil, out, nil
}
