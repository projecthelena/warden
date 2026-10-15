package operationmetrics

import (
	"errors"
	"github.com/projecthelena/warden/internal/metricstate"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestMetricsScrapeWhileRunningAndAfterFailure(t *testing.T) {
	finish := Start(BaselineCycle)
	scrape := func() string {
		t.Helper()
		response := httptest.NewRecorder()
		promhttp.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
		if response.Code != 200 {
			t.Fatalf("scrape status %d", response.Code)
		}
		return response.Body.String()
	}
	if !strings.Contains(scrape(), `warden_db_operations_in_flight{operation="baseline_cycle"} 1`) {
		t.Fatal("running cycle not exposed")
	}
	finish(errors.New("private driver failure"))
	body := scrape()
	for _, want := range []string{
		`warden_db_operations_in_flight{operation="baseline_cycle"} 0`,
		`warden_db_operation_duration_seconds_count{operation="baseline_cycle",outcome="error"} 1`,
		`warden_db_operation_last_started_timestamp_seconds{operation="baseline_cycle"}`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(body, "private driver failure") {
		t.Fatal("error text leaked")
	}
}

func TestConcurrentOperations(t *testing.T) {
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() { defer wg.Done(); finish := Start(PersistCommit); finish(nil) }()
	}
	wg.Wait()
	response := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	if !strings.Contains(response.Body.String(), `warden_db_operations_in_flight{operation="persist_commit"} 0`) {
		t.Fatal("concurrent operations leaked active work")
	}
}

func TestDisabledOperationsDoNotCreateSeries(t *testing.T) {
	metricstate.Configure(false)
	t.Cleanup(func() { metricstate.Configure(true) })
	finish := Start(Operation("disabled_test"))
	finish(errors.New("not recorded"))
	response := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	if strings.Contains(response.Body.String(), "disabled_test") {
		t.Fatal("disabled operation created a series")
	}
	if n := testing.AllocsPerRun(100, func() { done := Start(BaselineCount); done(nil) }); n != 0 {
		t.Fatalf("disabled operation allocated %v times", n)
	}
}
