package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lib/pq"
)

func TestTraceOperationRedaction(t *testing.T) {
	for _, query := range []string{
		"SELECT 'secret-password'", "INSERT INTO settings VALUES ('token','secret-password')",
		"/* secret-password */ SELECT 1", "SELECT COUNT(*) FROM monitor_checks WHERE monitor_id='secret-password'",
	} {
		if strings.Contains(traceOperation(query), "secret-password") {
			t.Fatal("query content escaped")
		}
	}
	var events []traceEvent
	c := &traceConn{owner: &traceConnector{run: "test", emit: func(e traceEvent) { events = append(events, e) }}, id: 1, pid: 42}
	_, finish := c.start("checks_insert", 0)
	finish(&pq.Error{Code: "23505", Message: "secret-password", Detail: "private-token"})
	finish(nil)
	if len(events) != 2 || events[1].SQLState != "23505" || events[1].Outcome != "error" {
		t.Fatalf("events: %+v", events)
	}
	data, _ := json.Marshal(events)
	if strings.Contains(string(data), "secret-password") || strings.Contains(string(data), "private-token") {
		t.Fatal("error detail escaped")
	}
}

func TestPostgresTraceIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_URL required")
	}
	connector, err := pq.NewConnector(dsn)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var events []traceEvent
	snapshot := func() []traceEvent { mu.Lock(); defer mu.Unlock(); return append([]traceEvent(nil), events...) }
	c := &traceConnector{Connector: connector, run: "trace-test", emit: func(e traceEvent) { mu.Lock(); events = append(events, e); mu.Unlock() }}
	db := sql.OpenDB(c)
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	observer, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = observer.Close() }()

	// Observe an operation while blocked, using its exact tag and backend PID.
	done := make(chan error, 1)
	go func() { _, err := db.Exec("SELECT pg_sleep(0.5)"); done <- err }()
	var matched bool
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range snapshot() {
			if e.Phase != "start" {
				continue
			}
			var query, wait string
			err = observer.QueryRow("SELECT query, coalesce(wait_event,'') FROM pg_stat_activity WHERE pid=$1", e.PID).Scan(&query, &wait)
			if err == nil && strings.HasPrefix(query, fmt.Sprintf("/*warden:trace-test:%d*/", e.Span)) && wait == "PgSleep" {
				matched = true
				break
			}
		}
		if matched {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Fatal("could not correlate in-flight Warden span to PostgreSQL wait")
	}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	stmt, err := tx.Prepare("SELECT $1::bigint")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []int64{7, 8} {
		var got int64
		if err := stmt.QueryRow(want).Scan(&got); err != nil || got != want {
			t.Fatalf("prepared: got %d, err %v", got, err)
		}
	}
	_ = stmt.Close()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	tx, err = db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	rows, err := db.Query("SELECT 1::bigint; SELECT 2::bigint")
	if err != nil {
		t.Fatal(err)
	}
	types, err := rows.ColumnTypes()
	if err != nil || types[0].DatabaseTypeName() != "INT8" {
		t.Fatalf("column metadata: %v, %v", types, err)
	}
	for n := 1; n <= 2; n++ {
		if !rows.Next() {
			t.Fatal("missing row")
		}
		var got int
		if err := rows.Scan(&got); err != nil || got != n {
			t.Fatalf("row %d: %d, %v", n, got, err)
		}
		if rows.Next() {
			t.Fatal("unexpected extra row")
		}
		if n == 1 && !rows.NextResultSet() {
			t.Fatal("missing result set")
		}
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := db.ExecContext(ctx, "SELECT pg_sleep(5)"); err == nil {
		t.Fatal("cancellation ignored")
	}
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("SELECT 1/0"); err == nil {
		t.Fatal("SQL error lost")
	}
	starts := make(map[uint64]traceEvent)
	ends := make(map[uint64]traceEvent)
	var commit, rollback, prepared, sqlError bool
	for _, e := range snapshot() {
		if e.PID <= 0 || e.Connection == 0 {
			t.Fatalf("missing connection identity: %+v", e)
		}
		if e.Phase == "start" {
			starts[e.Span] = e
			continue
		}
		if _, exists := ends[e.Span]; exists {
			t.Fatal("duplicate finish")
		}
		ends[e.Span] = e
		commit = commit || e.Operation == "commit" && e.Transaction != 0
		rollback = rollback || e.Operation == "rollback" && e.Transaction != 0
		prepared = prepared || e.Statement != 0 && e.Transaction != 0
		sqlError = sqlError || e.SQLState == "22012"
	}
	if len(starts) != len(ends) || !commit || !rollback || !prepared || !sqlError {
		t.Fatalf("incomplete traces: starts=%d ends=%d commit=%v rollback=%v prepared=%v error=%v", len(starts), len(ends), commit, rollback, prepared, sqlError)
	}
}

func TestPostgresTraceStore(t *testing.T) {
	cfg := NewPostgresTestConfig()
	if cfg == nil {
		t.Skip("TEST_POSTGRES_URL required")
	}
	cfg.PostgresTrace = true
	cfg.IncrementalUptime = true
	s, err := NewStore(*cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	defer func() { _ = s.Reset() }()
	if _, err := s.db.Exec("DELETE FROM monitors; DELETE FROM groups"); err != nil {
		t.Fatal(err)
	}
	incrementalMonitor(t, s)
	batch := []CheckResult{checkAt("m1", "up", 0), checkAt("m1", "down", 0)}
	if err := s.BatchInsertChecks(batch); err != nil {
		t.Fatal(err)
	}
	if err := s.BatchInsertChecks(batch); err != nil {
		t.Fatal(err)
	}
	assertIncrementalCounts(t, s, batch[0].Timestamp.UTC().Format("2006-01-02"), 2, 1)
	if _, err := s.ComputeLatencyBaseline("m1", 24*time.Hour, 1, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkPostgresTrace(b *testing.B) {
	base := NewPostgresTestConfig()
	if base == nil {
		b.Skip("TEST_POSTGRES_URL required")
	}
	for _, enabled := range []bool{false, true} {
		b.Run(fmt.Sprintf("enabled=%t", enabled), func(b *testing.B) {
			output, err := os.CreateTemp(b.TempDir(), "trace-*.log")
			if err != nil {
				b.Fatal(err)
			}
			previous := log.Writer()
			log.SetOutput(output)
			defer func() { log.SetOutput(previous); _ = output.Close() }()
			cfg := *base
			cfg.PostgresTrace = enabled
			cfg.IncrementalUptime = true
			s, err := NewStore(cfg)
			if err != nil {
				b.Fatal(err)
			}
			defer func() { _ = s.Close() }()
			if err = s.Reset(); err != nil {
				b.Fatal(err)
			}
			for i := 0; i < 50; i++ {
				if err = s.CreateMonitor(Monitor{ID: fmt.Sprintf("trace-%d", i), GroupID: "g-default", Name: "Trace", Interval: 10}); err != nil {
					b.Fatal(err)
				}
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				batch := make([]CheckResult, 50)
				for j := range batch {
					batch[j] = CheckResult{MonitorID: fmt.Sprintf("trace-%d", j), Status: "up", Timestamp: time.Now().UTC()}
				}
				if err = s.BatchInsertChecks(batch); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
		})
	}
}
