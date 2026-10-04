package uptime

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

func lastHTTPHop(d *db.HTTPDiagnostics) *db.HTTPHop {
	if d == nil || len(d.Attempts) == 0 {
		return nil
	}
	a := d.Attempts[len(d.Attempts)-1]
	if len(a.Hops) == 0 {
		return nil
	}
	return &a.Hops[len(a.Hops)-1]
}

func (m *Manager) recentDiagnostics(id string, now time.Time) *db.HTTPDiagnostics {
	m.mu.RLock()
	mon := m.monitors[id]
	m.mu.RUnlock()
	if mon == nil {
		return nil
	}
	mon.mu.RLock()
	defer mon.mu.RUnlock()
	if now.Sub(mon.diagnosticAt) > 5*time.Minute || now.Before(mon.diagnosticAt) {
		return nil
	}
	return mon.lastDiagnostics
}

func diagnosticSummary(d *db.HTTPDiagnostics) string {
	h := lastHTTPHop(d)
	if h == nil {
		return ""
	}
	var result string
	if h.FailurePhase != "" {
		result = "Observed failure phase: " + h.FailurePhase + "."
	} else if h.StatusCode > 0 {
		result = fmt.Sprintf("The destination returned HTTP %d.", h.StatusCode)
	}
	for _, p := range h.Phases {
		if p.Name != "connection_acquire" {
			result += fmt.Sprintf(" %s %.1fms", p.Name, p.DurationMS)
		}
		if len(result) > 350 {
			break
		}
	}
	if d.External != nil {
		switch d.External.Outcome {
		case "response":
			result += " A second probe received an HTTP response; compare its configured target and network path."
		case "failed":
			result += " The second probe also failed; a shared network or destination problem remains possible."
		default:
			result += " No usable second-probe observation."
		}
	}
	return result
}

// Hosts are distinct names, not proof of independent networks or providers. Queue
// pressure also weakens attribution; neither signal identifies an ISP as the cause.
func (m *Manager) connectivityEvidence(outages []db.OpenOutage, now time.Time) string {
	phases := map[string]map[string]bool{}
	var simultaneous []db.OpenOutage
	ordered := append([]db.OpenOutage(nil), outages...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].StartTime.Before(ordered[j].StartTime) })
	for _, c := range cluster(ordered, 5*time.Minute) {
		if distinctMonitors(c) > distinctMonitors(simultaneous) {
			simultaneous = c
		}
	}
	for _, o := range simultaneous {
		h := lastHTTPHop(m.recentDiagnostics(o.MonitorID, now))
		if h == nil || h.Host == "" {
			continue
		}
		switch h.FailurePhase {
		case "dns", "tcp", "tls":
		default:
			continue
		}
		if phases[h.FailurePhase] == nil {
			phases[h.FailurePhase] = map[string]bool{}
		}
		phases[h.FailurePhase][strings.ToLower(h.Host)] = true
	}
	queueBusy := (cap(m.jobQueue) > 0 && len(m.jobQueue)*2 >= cap(m.jobQueue)) || (cap(m.resultQueue) > 0 && len(m.resultQueue)*2 >= cap(m.resultQueue))
	if queueBusy {
		return "Warden has queue pressure; local overload may affect observations. Connectivity cause is undetermined."
	}
	var evidence []string
	for phase, hosts := range phases {
		if len(hosts) >= 3 {
			evidence = append(evidence, fmt.Sprintf("%s failures across %d distinct hostnames", phase, len(hosts)))
		}
	}
	sort.Strings(evidence)
	if len(evidence) == 0 {
		return "Simultaneous failures alone do not identify a network or service cause."
	}
	return "Possible connectivity problem (limited confidence): " + strings.Join(evidence, "; ") + ". Queues are below half capacity; CPU, resolver health and network independence are not established. Shared infrastructure or destination failures remain possible; this does not identify the ISP."
}
