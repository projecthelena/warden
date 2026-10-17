package uptime

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
	"github.com/projecthelena/warden/internal/metricstate"
	"github.com/projecthelena/warden/internal/observability"
)

func TestPersistenceDrainsOnStop(t *testing.T) {
	for _, cfg := range db.GetTestConfigs(t) {
		t.Run(cfg.Name, func(t *testing.T) {
			cfg.Config.IncrementalUptime = true
			store, err := db.NewStore(cfg.Config)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = store.Close() }()
			id := "drain-checks"
			_ = store.DeleteMonitor(id)
			if err = store.CreateMonitor(db.Monitor{ID: id, GroupID: "g-default", Name: "Drain", Interval: 10}); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = store.DeleteMonitor(id) }()
			m := NewManager(store)
			// Exercise the production processor without scheduler or alert side effects.
			m.processorStarted.Store(true)
			m.wg.Add(1)
			go m.resultProcessor()
			for i := 0; i < 75; i++ {
				m.resultQueue <- CheckResult{MonitorID: id, Status: i%2 == 0, Timestamp: time.Now().UTC()}
			}
			m.Stop()
			m.Stop()
			checks, err := store.GetMonitorChecks(id, 100)
			if err != nil || len(checks) != 75 {
				t.Fatalf("checks=%d err=%v", len(checks), err)
			}
			days, err := store.GetDailyUptimeStatsForMonitors([]string{id}, 1)
			if err != nil {
				t.Fatal(err)
			}
			if got := days[id][0]; got.Total != 75 || got.Up != 38 {
				t.Fatalf("summary=%+v", got)
			}
		})
	}
}

func TestPersistenceRetriesBusySQLite(t *testing.T) {
	metricstate.Configure(true)
	path := filepath.Join(t.TempDir(), "checks.sqlite") + "?_busy_timeout=1"
	config := db.NewTestConfigWithPath(path)
	config.IncrementalUptime = true
	store, err := db.NewStore(config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	if err = store.CreateMonitor(db.Monitor{ID: "m1", GroupID: "g-default", Name: "M", Interval: 10}); err != nil {
		t.Fatal(err)
	}
	locker, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = locker.Close() }()
	conn, err := locker.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err = conn.ExecContext(context.Background(), "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = conn.ExecContext(context.Background(), "ROLLBACK") }()
	m := NewManager(store)
	defer m.cancelPersistence()
	batch := []db.CheckResult{{MonitorID: "m1", Status: "up", Timestamp: time.Now().UTC()}}
	before := histogramCount(t, observability.PersistDuration.WithLabelValues("error"))
	done := make(chan error, 1)
	go func() { done <- m.persistBatch(batch) }()
	deadline := time.Now().Add(3 * time.Second)
	for histogramCount(t, observability.PersistDuration.WithLabelValues("error")) == before {
		if time.Now().After(deadline) {
			t.Fatal("busy write was not observed")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err = conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("retry did not recover")
	}
	if err = m.persistBatch(batch); err != nil {
		t.Fatal(err)
	}
	checks, err := store.GetMonitorChecks("m1", 10)
	if err != nil || len(checks) != 1 {
		t.Fatalf("checks=%d err=%v", len(checks), err)
	}
	days, err := store.GetDailyUptimeStatsForMonitors([]string{"m1"}, 1)
	if err != nil || days["m1"][0].Total != 1 {
		t.Fatalf("days=%v err=%v", days, err)
	}
}

func TestPersistencePermanentErrorStopsProcessor(t *testing.T) {
	cfg := db.NewTestConfig()
	cfg.IncrementalUptime = true
	store, err := db.NewStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(store)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	m.processorStarted.Store(true)
	m.wg.Add(1)
	go m.resultProcessor()
	for i := 0; i < BatchSize; i++ {
		m.resultQueue <- CheckResult{MonitorID: "m1", Status: true, Timestamp: time.Now().UTC()}
	}
	select {
	case <-m.processorDone:
	case <-time.After(3 * time.Second):
		t.Fatal("permanent failure retried indefinitely")
	}
	if m.persistCtx.Err() == nil {
		t.Fatal("workers were not cancelled")
	}
	m.Stop()
}

func TestPersistenceRetriesPostgresSerialization(t *testing.T) {
	cfg := db.NewPostgresTestConfig()
	if cfg == nil {
		t.Skip("TEST_POSTGRES_URL is not set")
	}
	cfg.IncrementalUptime = true
	store, err := db.NewStore(*cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	const id = "retry-serialization"
	_ = store.DeleteMonitor(id)
	if err = store.CreateMonitor(db.Monitor{ID: id, GroupID: "g-default", Name: "Retry", Interval: 10}); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.DeleteMonitor(id) }()
	conn, err := sql.Open("postgres", cfg.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	// A sequence survives transaction rollback, so exactly the first attempt fails.
	_, err = conn.Exec(`CREATE SEQUENCE test_check_attempt;
 CREATE FUNCTION test_check_serialization() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN
  IF nextval('test_check_attempt') = 1 THEN
   RAISE EXCEPTION 'injected serialization failure' USING ERRCODE = '40001';
  END IF;
  RETURN NEW;
 END $$;
 CREATE TRIGGER test_check_failure BEFORE INSERT ON monitor_checks
 FOR EACH ROW EXECUTE FUNCTION test_check_serialization();`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = conn.Exec(`DROP TRIGGER test_check_failure ON monitor_checks; DROP FUNCTION test_check_serialization(); DROP SEQUENCE test_check_attempt;`)
	}()
	m := NewManager(store)
	defer m.cancelPersistence()
	batch := []db.CheckResult{{MonitorID: id, Status: "up", Timestamp: time.Now().UTC()}}
	if err = m.persistBatch(batch); err != nil {
		t.Fatal(err)
	}
	if err = m.persistBatch(batch); err != nil {
		t.Fatal(err)
	}
	checks, err := store.GetMonitorChecks(id, 10)
	if err != nil || len(checks) != 1 {
		t.Fatalf("checks=%d err=%v", len(checks), err)
	}
	days, err := store.GetDailyUptimeStatsForMonitors([]string{id}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := days[id][0]; got.Total != 1 || got.Up != 1 {
		t.Fatalf("summary=%+v", got)
	}
}
