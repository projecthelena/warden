package api

import (
	"errors"
	"fmt"
	"strings"

	"github.com/projecthelena/warden/internal/db"
)

// The HTTP handler and the MCP tool both create monitors. They share this so the rules
// cannot drift apart: the two callers differ only in how they report the failure.
var (
	ErrMonitorInvalid   = errors.New("invalid monitor")
	ErrGroupNotFound    = errors.New("group not found")
	ErrMonitorNameTaken = errors.New("monitor name already exists")
)

// MonitorInput is everything needed to create a monitor.
type MonitorInput struct {
	Name                    string
	Type                    string
	URL                     string
	GroupID                 string
	Interval                int
	ConfirmationThreshold   *int
	NotificationCooldownMin *int
	LatencyThreshold        *int
	RequestConfig           *db.RequestConfig
}

// AddMonitor validates the input, stores the monitor and puts it into rotation.
// Errors wrap one of the sentinels above so callers can map them to a status code.
func (h *CRUDHandler) AddMonitor(in MonitorInput) (db.Monitor, error) {
	if in.Name == "" || in.URL == "" || in.GroupID == "" {
		return db.Monitor{}, fmt.Errorf("%w: name, url and groupId are required", ErrMonitorInvalid)
	}
	if len(in.Name) > maxNameLength {
		return db.Monitor{}, fmt.Errorf("%w: name too long (max %d characters)", ErrMonitorInvalid, maxNameLength)
	}

	if in.Type != "" && !db.IsValidMonitorType(in.Type) {
		return db.Monitor{}, fmt.Errorf("%w: type must be one of http, tcp, ping, dns", ErrMonitorInvalid)
	}
	if err := validateTarget(in.Type, in.URL); err != nil {
		return db.Monitor{}, fmt.Errorf("%w: %s", ErrMonitorInvalid, err)
	}
	if in.Interval < 10 {
		return db.Monitor{}, fmt.Errorf("%w: interval must be at least 10 seconds", ErrMonitorInvalid)
	}

	if err := validateNotificationOverrides(in); err != nil {
		return db.Monitor{}, err
	}
	if err := validateRequestConfig(in.RequestConfig); err != nil {
		return db.Monitor{}, fmt.Errorf("%w: %s", ErrMonitorInvalid, err)
	}

	if err := h.requireGroup(in.GroupID); err != nil {
		return db.Monitor{}, err
	}

	// Simulates a unique constraint the schema does not have.
	if monitors, err := h.store.GetMonitors(); err == nil {
		for _, m := range monitors {
			if strings.EqualFold(m.Name, in.Name) {
				return db.Monitor{}, fmt.Errorf("%w: %s", ErrMonitorNameTaken, in.Name)
			}
		}
	}

	m := db.Monitor{
		ID:                      generateID(in.Name, "m-"),
		Type:                    db.NormalizeMonitorType(in.Type),
		GroupID:                 in.GroupID,
		Name:                    in.Name,
		URL:                     in.URL,
		Active:                  true,
		Interval:                in.Interval,
		ConfirmationThreshold:   in.ConfirmationThreshold,
		NotificationCooldownMin: in.NotificationCooldownMin,
		LatencyThreshold:        in.LatencyThreshold,
		RequestConfig:           in.RequestConfig,
	}
	if err := h.store.CreateMonitor(m); err != nil {
		return db.Monitor{}, err
	}

	h.manager.Sync()
	return m, nil
}

// requireGroup fails unless the group exists. group_id is a foreign key, so without this
// the caller would get a constraint error instead of a "no such group".
func (h *CRUDHandler) requireGroup(groupID string) error {
	if groupID == "" {
		return fmt.Errorf("%w: groupId is required", ErrMonitorInvalid)
	}
	groups, err := h.store.GetGroups()
	if err != nil {
		return fmt.Errorf("failed to check groups: %w", err)
	}
	for _, g := range groups {
		if g.ID == groupID {
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrGroupNotFound, groupID)
}

func validateNotificationOverrides(in MonitorInput) error {
	if in.ConfirmationThreshold != nil && (*in.ConfirmationThreshold < 1 || *in.ConfirmationThreshold > 100) {
		return fmt.Errorf("%w: confirmationThreshold must be between 1 and 100", ErrMonitorInvalid)
	}
	if in.NotificationCooldownMin != nil && (*in.NotificationCooldownMin < 0 || *in.NotificationCooldownMin > 1440) {
		return fmt.Errorf("%w: notificationCooldownMinutes must be between 0 and 1440", ErrMonitorInvalid)
	}
	if in.LatencyThreshold != nil && *in.LatencyThreshold < 1 {
		return fmt.Errorf("%w: latencyThreshold must be at least 1", ErrMonitorInvalid)
	}
	return nil
}

// AddGroup validates and stores a group. Shared with the MCP tool for the same reason.
func (h *CRUDHandler) AddGroup(name string) (db.Group, error) {
	if name == "" {
		return db.Group{}, fmt.Errorf("%w: name is required", ErrMonitorInvalid)
	}
	if len(name) > maxNameLength {
		return db.Group{}, fmt.Errorf("%w: name too long (max %d characters)", ErrMonitorInvalid, maxNameLength)
	}

	g := db.Group{ID: generateSlug(name, "g-"), Name: name}
	if err := h.store.CreateGroup(g); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "duplicate key") {
			return db.Group{}, fmt.Errorf("group %q already exists", name)
		}
		return db.Group{}, err
	}
	return g, nil
}

// The MCP server drives creation through these, so both surfaces validate identically.
// The adapter lives here because the shape belongs to the MCP package, not the API.

func (h *CRUDHandler) SetMonitorActive(id string, active bool) error {
	if err := h.store.SetMonitorActive(id, active); err != nil {
		return err
	}
	h.manager.Sync()
	return nil
}

// MoveMonitor reassigns a monitor to another group. Grouping is the only thing that
// changes: history, uptime and open incidents stay attached to the monitor.
func (h *CRUDHandler) MoveMonitor(monitorID, groupID string) error {
	if monitorID == "" {
		return fmt.Errorf("%w: monitor id is required", ErrMonitorInvalid)
	}
	if err := h.requireGroup(groupID); err != nil {
		return err
	}
	if err := h.store.MoveMonitorToGroup(monitorID, groupID); err != nil {
		return err
	}
	h.manager.Sync()
	return nil
}

func (h *CRUDHandler) RenameGroup(id, name string) error {
	if name == "" {
		return fmt.Errorf("%w: name is required", ErrMonitorInvalid)
	}
	if len(name) > maxNameLength {
		return fmt.Errorf("%w: name too long (max %d characters)", ErrMonitorInvalid, maxNameLength)
	}
	return h.store.UpdateGroup(id, name)
}
