package api

import (
	"github.com/projecthelena/warden/internal/db"
	wardenmcp "github.com/projecthelena/warden/internal/mcp"
)

// mcpWriter lets the MCP tools reach the same creation logic the HTTP handlers use. The
// two packages keep their own input types so neither has to import the other's; this is
// the seam between them.
type mcpWriter struct{ h *CRUDHandler }

func (w mcpWriter) AddMonitor(in wardenmcp.MonitorInput) (db.Monitor, error) {
	return w.h.AddMonitor(MonitorInput{
		Name:     in.Name,
		Type:     in.Type,
		URL:      in.URL,
		GroupID:  in.GroupID,
		Interval: in.Interval,
	})
}

func (w mcpWriter) AddGroup(name string) (db.Group, error) { return w.h.AddGroup(name) }

func (w mcpWriter) SetMonitorActive(id string, active bool) error {
	return w.h.SetMonitorActive(id, active)
}

func (w mcpWriter) RenameGroup(id, name string) error { return w.h.RenameGroup(id, name) }

func (w mcpWriter) MoveMonitor(monitorID, groupID string) error {
	return w.h.MoveMonitor(monitorID, groupID)
}
