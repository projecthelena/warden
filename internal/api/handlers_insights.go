package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/projecthelena/warden/internal/db"
)

type InsightHandler struct {
	store       *db.Store
	captureHTTP bool
}

func NewInsightHandler(store *db.Store, captureHTTP bool) *InsightHandler {
	return &InsightHandler{store: store, captureHTTP: captureHTTP}
}

// GetInsights returns the pattern findings, optionally for a single monitor.
//
// These answer a different question from incidents. An incident says something broke;
// a finding says something about the shape of how it keeps breaking — that it climbs and
// resets, that it only misbehaves in the evening, that it always fails alongside another
// monitor.
// @Summary      List pattern findings
// @Tags         insights
// @Produce      json
// @Security     BearerAuth
// @Param        monitorId query string false "Limit to one monitor"
// @Success      200 {array} db.MonitorInsight
// @Router       /insights [get]
func (h *InsightHandler) GetInsights(w http.ResponseWriter, r *http.Request) {
	monitorID := r.URL.Query().Get("monitorId")

	findings, err := h.store.GetMonitorInsights(monitorID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load insights")
		return
	}
	if findings == nil {
		findings = []db.MonitorInsight{}
	}
	if len(findings) > 0 {
		if err := h.addTraceCapture(findings); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load trace capture policy")
			return
		}
	}
	writeJSON(w, http.StatusOK, findings)
}

// GetMonitorInsights returns the pattern findings for one monitor.
// @Summary      Pattern findings for a monitor
// @Tags         insights
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Monitor ID"
// @Success      200 {array} db.MonitorInsight
// @Router       /monitors/{id}/insights [get]
func (h *InsightHandler) GetMonitorInsights(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "ID required")
		return
	}

	findings, err := h.store.GetMonitorInsights(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load insights")
		return
	}
	if findings == nil {
		findings = []db.MonitorInsight{}
	}
	if len(findings) > 0 {
		if err := h.addTraceCapture(findings); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load trace capture policy")
			return
		}
	}
	writeJSON(w, http.StatusOK, findings)
}

func (h *InsightHandler) addTraceCapture(findings []db.MonitorInsight) error {
	monitors, err := h.store.GetMonitors()
	if err != nil {
		return err
	}
	policies := make(map[string]string, len(monitors))
	for _, monitor := range monitors {
		policy := "disabled"
		if db.NormalizeMonitorType(monitor.Type) != db.MonitorTypeHTTP {
			policy = "not_applicable"
		} else if h.captureHTTP || (monitor.RequestConfig != nil && monitor.RequestConfig.AutoRetry) {
			policy = "enabled"
		}
		policies[monitor.ID] = policy
	}
	for i := range findings {
		findings[i].TraceCapture = policies[findings[i].MonitorID]
	}
	return nil
}
