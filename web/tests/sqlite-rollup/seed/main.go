package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

func main() {
	if err := seed(); err != nil {
		log.Fatal(err)
	}
}

func seed() error {
	path := os.Getenv("DB_PATH")
	if path == "" {
		return fmt.Errorf("DB_PATH is required")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return fmt.Errorf("fixture requires a new database")
	}
	store, err := db.NewStore(db.DBConfig{Type: db.DialectSQLite, Path: path})
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	if err := store.CreateUser("rollup-fixture", rand.Text(), "UTC", "admin"); err != nil {
		return err
	}
	if err := store.CreateGroup(db.Group{ID: "rollup", Name: "Rollup services"}); err != nil {
		return err
	}
	// No precomputed rows: the production startup worker must build this history.
	now := time.Now().UTC()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	var checks []db.CheckResult
	for i := 0; i < 123; i++ {
		id := fmt.Sprintf("rollup-%03d", i)
		if err := store.CreateMonitor(db.Monitor{ID: id, Name: fmt.Sprintf("Rollup service %03d", i), GroupID: "rollup", Type: db.MonitorTypeHTTP, URL: "https://example.invalid", Interval: 60, Active: false}); err != nil {
			return err
		}
		for _, days := range []int{1, 3} {
			for n := 0; n < 4; n++ {
				status := "up"
				if n == 0 {
					status = "down"
				}
				checks = append(checks, db.CheckResult{MonitorID: id, Status: status, Timestamp: midnight.AddDate(0, 0, -days).Add(time.Duration(n+1) * time.Hour), Diagnostics: &db.HTTPDiagnostics{}})
			}
		}
	}
	if err := store.BatchInsertChecks(checks); err != nil {
		return err
	}
	return store.UpsertStatusPage("rollup", "Rollup history", nil, true, true)
}
