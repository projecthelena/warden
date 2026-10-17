package e2e

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

// Run against a built dashboard binary. E2E_POSTGRES_URL must be a disposable DB.
func TestIncrementalUptimeProcessCycle(t *testing.T) {
	binary := os.Getenv("WARDEN_E2E_BINARY")
	if binary == "" {
		t.Skip("WARDEN_E2E_BINARY is not set")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	configs := []db.DBConfig{{Type: db.DialectSQLite, Path: filepath.Join(t.TempDir(), "warden.db")}}
	if url := os.Getenv("E2E_POSTGRES_URL"); url != "" {
		configs = append(configs, db.DBConfig{Type: db.DialectPostgres, URL: url})
	}
	for _, cfg := range configs {
		t.Run(cfg.Type, func(t *testing.T) {
			var down atomic.Bool
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if down.Load() {
					w.WriteHeader(503)
				} else {
					w.WriteHeader(200)
				}
			}))
			defer target.Close()
			store, err := db.NewStore(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err = store.Reset(); err != nil {
				t.Fatal(err)
			}
			if err = store.CreateUser("e2e-fixture", rand.Text(), "UTC", "admin"); err != nil {
				t.Fatal(err)
			}
			if err = store.CreateMonitor(db.Monitor{ID: "probe", GroupID: "g-default", Name: "Probe", Type: db.MonitorTypeHTTP, URL: target.URL, Interval: 1, Active: true}); err != nil {
				t.Fatal(err)
			}
			if err = store.UpsertStatusPage("counters", "Counters", nil, true, true); err != nil {
				t.Fatal(err)
			}
			if err = store.Close(); err != nil {
				t.Fatal(err)
			}
			driver, dsn := "sqlite3", cfg.Path+"?_busy_timeout=5000"
			if cfg.Type == db.DialectPostgres {
				driver, dsn = "postgres", cfg.URL
			}
			reader, err := sql.Open(driver, dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = reader.Close() }()
			counts := func() (total, up int) {
				t.Helper()
				if err := reader.QueryRow("SELECT count(*),COALESCE(sum(CASE WHEN status='up' THEN 1 ELSE 0 END),0) FROM monitor_checks").Scan(&total, &up); err != nil {
					t.Fatal(err)
				}
				return
			}
			client := &http.Client{Timeout: time.Second}
			for phase, enabled := range []bool{false, true, true, false, true} {
				t.Run(fmt.Sprintf("%d_enabled_%t", phase, enabled), func(t *testing.T) {
					before, beforeUp := counts()
					down.Store(phase%2 == 1)
					listener, err := net.Listen("tcp", "127.0.0.1:0")
					if err != nil {
						t.Fatal(err)
					}
					addr := listener.Addr().String()
					_ = listener.Close()
					logFile, err := os.CreateTemp(t.TempDir(), "server-*.log")
					if err != nil {
						t.Fatal(err)
					}
					defer func() { _ = logFile.Close() }()
					cmd := exec.Command(binary)
					overrides := map[string]string{"DB_TYPE": cfg.Type, "DB_PATH": cfg.Path, "DB_URL": cfg.URL, "INCREMENTAL_UPTIME_ENABLED": strconv.FormatBool(enabled), "LISTEN_ADDR": addr, "OBSERVABILITY_ADDR": "", "COOKIE_SECURE": "false", "SECONDARY_PROBE_URL": ""}
					for _, entry := range os.Environ() {
						key, _, _ := strings.Cut(entry, "=")
						if _, ok := overrides[key]; !ok {
							cmd.Env = append(cmd.Env, entry)
						}
					}
					for key, value := range overrides {
						cmd.Env = append(cmd.Env, key+"="+value)
					}
					cmd.Stdout = logFile
					cmd.Stderr = logFile
					if err = cmd.Start(); err != nil {
						t.Fatal(err)
					}
					stopped := false
					stop := func() {
						t.Helper()
						if stopped {
							return
						}
						stopped = true
						// Also prove committed counts survive an abrupt process death.
						if phase == 2 {
							_ = cmd.Process.Kill()
						} else {
							_ = cmd.Process.Signal(syscall.SIGTERM)
						}
						done := make(chan error, 1)
						go func() { done <- cmd.Wait() }()
						select {
						case err := <-done:
							if err != nil && phase != 2 {
								t.Errorf("server exit: %v", err)
							}
						case <-time.After(15 * time.Second):
							_ = cmd.Process.Kill()
							<-done
							t.Error("server shutdown timed out")
						}
					}
					defer func() {
						stop()
						if t.Failed() {
							data, _ := os.ReadFile(logFile.Name())
							t.Log(string(data))
						}
					}()
					waitFor(t, func() bool {
						resp, err := client.Get("http://" + addr + "/readyz")
						if err != nil {
							return false
						}
						_ = resp.Body.Close()
						return resp.StatusCode == 200
					})
					// Real scheduler → HTTP probe → batch writer, including both up and down checks.
					waitFor(t, func() bool { n, _ := counts(); return n >= before+2 })
					if enabled {
						waitFor(t, func() bool {
							resp, err := client.Get("http://" + addr + "/api/s/counters?group=g-default&page_size=100")
							if err != nil {
								return false
							}
							defer func() { _ = resp.Body.Close() }()
							var body struct {
								Groups []struct {
									Monitors []struct {
										ID   string               `json:"id"`
										Days []db.DailyUptimeStat `json:"uptimeDays"`
									} `json:"monitors"`
								} `json:"groups"`
							}
							if resp.StatusCode != 200 || json.NewDecoder(resp.Body).Decode(&body) != nil {
								return false
							}
							for _, group := range body.Groups {
								for _, monitor := range group.Monitors {
									if monitor.ID == "probe" {
										total := 0
										for _, day := range monitor.Days {
											total += day.Total
										}
										return total >= before+2
									}
								}
							}
							return false
						})
					}
					stop()
					total, up := counts()
					expectedUp := total - before
					if down.Load() {
						expectedUp = 0
					}
					if up-beforeUp != expectedUp {
						t.Fatalf("probe outcome: up delta=%d, want %d", up-beforeUp, expectedUp)
					}
					var dailyTotal, dailyUp, identified int
					if err := reader.QueryRow("SELECT COALESCE(sum(total),0),COALESCE(sum(up_count),0) FROM monitor_uptime_daily").Scan(&dailyTotal, &dailyUp); err != nil {
						t.Fatal(err)
					}
					if err := reader.QueryRow("SELECT count(*) FROM monitor_checks WHERE ingest_id IS NOT NULL").Scan(&identified); err != nil {
						t.Fatal(err)
					}
					if enabled && (dailyTotal != total || dailyUp != up) {
						t.Fatalf("counters %d/%d, raw %d/%d", dailyUp, dailyTotal, up, total)
					}
					if !enabled && dailyTotal >= total {
						t.Fatalf("legacy mode unexpectedly updated live counters: summary=%d raw=%d", dailyTotal, total)
					}
					if phase == 0 && identified != 0 {
						t.Fatal("default mode generated ingestion identities")
					}
					if enabled && identified == 0 {
						t.Fatal("incremental mode did not generate identities")
					}
				})
				if t.Failed() {
					return
				}
			}
		})
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}
