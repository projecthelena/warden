package db

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
	collector "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
)

func TestPostgresOTLP(t *testing.T) {
	received := make(chan *collector.ExportTraceServiceRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/insert/opentelemetry/v1/traces" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		request := new(collector.ExportTraceServiceRequest)
		if err := proto.Unmarshal(data, request); err != nil {
			t.Error(err)
		}
		received <- request
		w.Header().Set("Content-Type", "application/x-protobuf")
	}))
	defer server.Close()
	provider, err := postgresTraceProvider(server.URL + "/insert/opentelemetry/v1/traces")
	if err != nil {
		t.Fatal(err)
	}
	var events []traceEvent
	owner := &traceConnector{run: "test-run", tracer: provider.Tracer("test"), emit: func(e traceEvent) { events = append(events, e) }}
	owner.sequence.Store(math.MaxUint64 - 1)
	conn := &traceConn{owner: owner, pid: 42, id: math.MaxUint64, transaction: math.MaxUint64}
	conn.txContext, conn.txSpan = owner.tracer.Start(context.Background(), "postgres.transaction")
	_, finish := conn.start("checks_insert", math.MaxUint64)
	finish(&pq.Error{Code: "23505", Message: "secret-query-value"})
	finish(nil)
	conn.endTransaction(nil)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := provider.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].TraceID == "" || events[0].SpanID == "" || events[0].TraceID != events[1].TraceID {
		t.Fatalf("invalid events: %+v", events)
	}
	select {
	case request := <-received:
		spans := request.ResourceSpans[0].ScopeSpans[0].Spans
		if len(spans) != 2 {
			t.Fatalf("got %d spans", len(spans))
		}
		if len(spans[0].ParentSpanId) != 8 || string(spans[0].TraceId) != string(spans[1].TraceId) {
			t.Fatal("missing transaction parent")
		}
		if spans[0].Status.Message != "error" {
			t.Fatal("expected sanitized error")
		}
		attributes := map[string]string{}
		for _, a := range spans[0].Attributes {
			attributes[a.Key] = a.Value.GetStringValue()
		}
		for _, key := range []string{"warden.span", "warden.transaction", "warden.statement", "warden.connection"} {
			if attributes[key] != "18446744073709551615" {
				t.Errorf("%s lost unsigned ID: %q", key, attributes[key])
			}
		}
		payload, _ := json.Marshal(request)
		if strings.Contains(string(payload), "secret-query-value") {
			t.Fatal("sensitive error exported")
		}
	case <-ctx.Done():
		t.Fatal("no OTLP request")
	}
}

func TestPostgresOTLPRejectsInvalidEndpoint(t *testing.T) {
	for _, endpoint := range []string{"ftp://example.com", "missing-host", "http://user:secret@example.com"} {
		if _, err := postgresTraceProvider(endpoint); err == nil {
			t.Fatalf("accepted %q", endpoint)
		}
	}
}

func TestPostgresOTLPSQLiteIgnoresEndpoint(t *testing.T) {
	store, err := NewStore(DBConfig{Type: DialectSQLite, Path: ":memory:", PostgresTrace: true, PostgresTraceEndpoint: "invalid"})
	if err != nil {
		t.Fatal(err)
	}
	if store.traceShutdown != nil {
		t.Fatal("SQLite initialized OTLP")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresOTLPDisabledIgnoresEndpoint(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_URL required")
	}
	store, err := NewStore(DBConfig{Type: DialectPostgres, URL: dsn, PostgresTraceEndpoint: "invalid"})
	if err != nil {
		t.Fatal(err)
	}
	if store.traceShutdown != nil {
		t.Fatal("disabled tracing initialized OTLP")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresOTLPUnavailableDoesNotBlockOperation(t *testing.T) {
	provider, err := postgresTraceProvider("http://127.0.0.1:1/insert/opentelemetry/v1/traces")
	if err != nil {
		t.Fatal(err)
	}
	conn := &traceConn{owner: &traceConnector{tracer: provider.Tracer("test"), emit: func(traceEvent) {}}}
	started := time.Now()
	_, finish := conn.start("checks_insert", 0)
	finish(nil)
	if time.Since(started) > time.Second {
		t.Fatal("operation waited for exporter")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = provider.Shutdown(ctx)
}
