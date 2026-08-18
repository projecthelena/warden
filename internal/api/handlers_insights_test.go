package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/projecthelena/warden/internal/db"
)

// withURLParam attaches a chi route parameter, which the handler reads via chi.URLParam.
func withURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func seedInsightFixture(t *testing.T) *db.Store {
	t.Helper()
	s, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := s.CreateGroup(db.Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if err := s.CreateMonitor(db.Monitor{
		ID: "m1", GroupID: "g1", Name: "API", URL: "https://api.example.com", Interval: 60,
	}); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}
	if err := s.ReplaceMonitorInsights("m1", []db.MonitorInsight{{
		Kind: "latency_sawtooth", Summary: "API climbs and resets", Confidence: "high",
		Detail: map[string]any{"ramps": 9},
	}}, time.Now().UTC()); err != nil {
		t.Fatalf("ReplaceMonitorInsights: %v", err)
	}
	return s
}

func TestGetMonitorInsights_ReturnsFindings(t *testing.T) {
	s := seedInsightFixture(t)
	h := NewInsightHandler(s)

	req := httptest.NewRequest("GET", "/api/monitors/m1/insights", nil)
	req = withURLParam(req, "id", "m1")
	w := httptest.NewRecorder()

	h.GetMonitorInsights(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var out []db.MonitorInsight
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(out) != 1 || out[0].Kind != "latency_sawtooth" {
		t.Fatalf("unexpected payload: %+v", out)
	}
	if out[0].Detail["ramps"] == nil {
		t.Errorf("detail did not survive: %+v", out[0].Detail)
	}
}

// A monitor with nothing to report must serialise as [] rather than null: the UI iterates
// the response directly.
func TestGetMonitorInsights_EmptyIsAnArray(t *testing.T) {
	s := seedInsightFixture(t)
	h := NewInsightHandler(s)

	req := httptest.NewRequest("GET", "/api/monitors/m-none/insights", nil)
	req = withURLParam(req, "id", "m-none")
	w := httptest.NewRecorder()

	h.GetMonitorInsights(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := w.Body.String(); got != "[]\n" && got != "[]" {
		t.Errorf("expected an empty array, got %q", got)
	}
}

func TestGetMonitorInsights_RequiresAnID(t *testing.T) {
	s := seedInsightFixture(t)
	h := NewInsightHandler(s)

	req := httptest.NewRequest("GET", "/api/monitors//insights", nil)
	req = withURLParam(req, "id", "")
	w := httptest.NewRecorder()

	h.GetMonitorInsights(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 without an id, got %d", w.Code)
	}
}

// GetInsights is what the Swagger annotation publishes as /api/insights. It was annotated
// but never routed, so the generated docs advertised an endpoint that 404s.
func TestGetInsights_ListsEveryMonitor(t *testing.T) {
	s := seedInsightFixture(t)
	h := NewInsightHandler(s)

	req := httptest.NewRequest("GET", "/api/insights", nil)
	w := httptest.NewRecorder()

	h.GetInsights(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var out []db.MonitorInsight
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(out) != 1 || out[0].MonitorName != "API" {
		t.Fatalf("unexpected payload: %+v", out)
	}
}

func TestGetInsights_FiltersByMonitor(t *testing.T) {
	s := seedInsightFixture(t)
	h := NewInsightHandler(s)

	req := httptest.NewRequest("GET", "/api/insights?monitorId=m-other", nil)
	w := httptest.NewRecorder()

	h.GetInsights(w, req)

	var out []db.MonitorInsight
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("filter returned another monitor's findings: %+v", out)
	}
}
