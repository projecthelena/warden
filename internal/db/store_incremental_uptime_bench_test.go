package db

import (
	"fmt"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// This isolates persistence cost; it is not the end-to-end monitor load test.
func BenchmarkIncrementalUptime(b *testing.B) {
	configs := []TestDBConfig{{Name: "SQLite", Config: NewTestConfig()}}
	if pg := NewPostgresTestConfig(); pg != nil {
		configs = append(configs, TestDBConfig{Name: "PostgreSQL", Config: *pg})
	}
	for _, cfg := range configs {
		for _, incremental := range []bool{false, true} {
			b.Run(fmt.Sprintf("%s/incremental=%t", cfg.Name, incremental), func(b *testing.B) {
				config := cfg.Config
				config.IncrementalUptime = incremental
				if config.Type == DialectSQLite {
					config.Path = filepath.Join(b.TempDir(), "load.sqlite")
				}
				s, err := NewStore(config)
				if err != nil {
					b.Fatal(err)
				}
				defer func() { _ = s.Close() }()
				if err = s.Reset(); err != nil {
					b.Fatal(err)
				}
				tx, err := s.db.Begin()
				if err != nil {
					b.Fatal(err)
				}
				for i := 0; i < 5000; i++ {
					id := fmt.Sprintf("load-%04d", i)
					if _, err = tx.Exec(s.rebind("INSERT INTO monitors (id,group_id,name,url,interval_seconds) VALUES (?, 'g-default', ?, '', 10)"), id, id); err != nil {
						_ = tx.Rollback()
						b.Fatal(err)
					}
				}
				if err = tx.Commit(); err != nil {
					b.Fatal(err)
				}
				seed := `WITH RECURSIVE days(day) AS (VALUES(1) UNION ALL SELECT day+1 FROM days WHERE day<20)
                 INSERT INTO monitor_checks (monitor_id,status,latency,timestamp)
                 SELECT id,'up',0,datetime('now','-' || day || ' days') FROM monitors CROSS JOIN days`
				day := "date(timestamp)"
				if s.IsPostgres() {
					seed = `INSERT INTO monitor_checks (monitor_id,status,latency,timestamp)
                  SELECT id,'up',0,(CURRENT_TIMESTAMP AT TIME ZONE 'UTC') - day * interval '1 day'
                  FROM monitors CROSS JOIN generate_series(1,20) AS days(day)`
					day = "TO_CHAR(timestamp,'YYYY-MM-DD')"
				}
				if _, err = s.db.Exec(seed); err != nil {
					b.Fatal(err)
				}
				if _, err = s.db.Exec("INSERT INTO monitor_uptime_daily (monitor_id,day,total,up_count) SELECT monitor_id," + day + ",count(*),count(*) FROM monitor_checks GROUP BY monitor_id," + day); err != nil {
					b.Fatal(err)
				}

				durations := make([]time.Duration, 0, b.N)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					batch := make([]CheckResult, 50)
					for j := range batch {
						batch[j] = CheckResult{MonitorID: fmt.Sprintf("load-%04d", (i*50+j)%5000), Status: "up", Timestamp: time.Now().UTC()}
					}
					start := time.Now()
					err = s.BatchInsertChecks(batch)
					if err != nil {
						b.Fatal(err)
					}
					durations = append(durations, time.Since(start))
				}
				b.StopTimer()
				sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
				b.ReportMetric(float64(b.N*50)/b.Elapsed().Seconds(), "checks/s")
				b.ReportMetric(float64(durations[(len(durations)-1)*99/100])/float64(time.Millisecond), "p99-ms")
				b.ReportMetric(float64(durations[len(durations)-1])/float64(time.Millisecond), "max-ms")
				var n int
				if err = s.db.QueryRow("SELECT count(*) FROM monitor_checks").Scan(&n); err != nil || n != 100000+b.N*50 {
					b.Fatalf("raw=%d err=%v", n, err)
				}
				if incremental {
					if err = s.db.QueryRow("SELECT sum(total) FROM monitor_uptime_daily").Scan(&n); err != nil || n != 100000+b.N*50 {
						b.Fatalf("summary=%d err=%v", n, err)
					}
				}
			})
		}
	}
}
