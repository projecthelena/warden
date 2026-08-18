package uptime

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

// correlationPolicy decides when several failing monitors are one event rather than
// several. All of it is thresholds over things Warden already knows: which outages are
// open, when each started, and how many monitors a group has.
type correlationPolicy struct {
	// Window is how close together two outages must start to be considered the same event.
	Window time.Duration
	// MinMonitors is the floor: fewer than this is never a correlated incident, however
	// small the group.
	MinMonitors int
	// GroupPercent is the share of a group that must be failing. Expressed as a percentage
	// so it still means something when a group grows from 12 monitors to 200.
	GroupPercent int
	// ProbePercent is the share of *every* monitor that must be failing before Warden
	// concludes the problem is its own vantage point rather than the targets.
	ProbePercent int
	// ChronicLimit is how many times one monitor may interrupt you inside ChronicWindow
	// before its individual alerts are collapsed into a single "unstable" notice.
	ChronicLimit  int
	ChronicWindow time.Duration
}

func defaultCorrelationPolicy() correlationPolicy {
	return correlationPolicy{
		Window:        5 * time.Minute,
		MinMonitors:   3,
		GroupPercent:  30,
		ProbePercent:  80,
		ChronicLimit:  3,
		ChronicWindow: 24 * time.Hour,
	}
}

// requiredForGroup is the number of simultaneously failing monitors that makes a group's
// trouble one incident. The absolute floor wins on small groups, the percentage on large
// ones — with 12 monitors both say roughly "three or four", with 200 only the percentage
// still means anything.
func (p correlationPolicy) requiredForGroup(groupSize int) int {
	byPercent := (groupSize*p.GroupPercent + 99) / 100 // ceil
	if byPercent > p.MinMonitors {
		return byPercent
	}
	return p.MinMonitors
}

// cluster groups outages that started close enough together to share a cause. Input must
// be sorted by StartTime ascending; each cluster is anchored on its earliest member, so a
// slow-rolling failure does not chain indefinitely into one giant cluster.
func cluster(outages []db.OpenOutage, window time.Duration) [][]db.OpenOutage {
	var out [][]db.OpenOutage
	for _, o := range outages {
		placed := false
		for i := range out {
			anchor := out[i][0].StartTime
			if o.StartTime.Sub(anchor) <= window {
				out[i] = append(out[i], o)
				placed = true
				break
			}
		}
		if !placed {
			out = append(out, []db.OpenOutage{o})
		}
	}
	return out
}

// distinctMonitors counts monitors, not outages: one monitor can hold both a down and a
// degraded row, and that is one thing being broken.
func distinctMonitors(outages []db.OpenOutage) int {
	seen := make(map[string]struct{}, len(outages))
	for _, o := range outages {
		seen[o.MonitorID] = struct{}{}
	}
	return len(seen)
}

// distinctGroups counts how many groups a set of outages spans. Trouble confined to one
// group has a group-shaped explanation; trouble across several points at something they
// share, which from Warden's vantage point is usually Warden's own network.
func distinctGroups(outages []db.OpenOutage) int {
	seen := make(map[string]struct{}, len(outages))
	for _, o := range outages {
		seen[o.GroupID] = struct{}{}
	}
	return len(seen)
}

func outageIDs(outages []db.OpenOutage) []int64 {
	ids := make([]int64, 0, len(outages))
	for _, o := range outages {
		ids = append(ids, o.ID)
	}
	return ids
}

// monitorList renders the affected monitors for a message, alphabetically and capped. Past
// a handful the names stop being information and start being a wall.
func monitorList(outages []db.OpenOutage, max int) string {
	names := make([]string, 0, len(outages))
	seen := make(map[string]struct{}, len(outages))
	for _, o := range outages {
		if _, dup := seen[o.MonitorName]; dup {
			continue
		}
		seen[o.MonitorName] = struct{}{}
		names = append(names, o.MonitorName)
	}
	sort.Strings(names)

	if len(names) <= max {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(names[:max], ", "), len(names)-max)
}

// correlationKey is the id shared by every outage announced in the same message, so the
// reminders that follow stay one message instead of one per monitor. Derived from the
// anchor outage rather than random, which keeps workflow runs reproducible and makes the
// id readable in the database.
func correlationKey(prefix string, anchor db.OpenOutage) string {
	return fmt.Sprintf("%s-%d-%d", prefix, anchor.ID, anchor.StartTime.UTC().Unix())
}
