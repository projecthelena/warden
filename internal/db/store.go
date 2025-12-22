package db

import (
	"database/sql"
	"embed"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

type Store struct {
	db *sql.DB
}

func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Enable Foreign Keys for Cascade Deletion
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, err
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}

	if err := s.seed(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	// Simple migration runner using embedded files.
	// We read 000001_init_schema.up.sql and execute it.
	// Since we use IF NOT EXISTS, it's idempotent for schema creation.

	// In the future, we can add a 'schema_migrations' table check here.

	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations db: %w", err)
	}

	for _, entry := range entries {
		// Only run .up.sql files for now
		// In a real runner, we'd sort by filename (000001, 000002) and track state.
		// For this refactor, we just run the init schema which contains everything.
		if entry.IsDir() {
			continue
		}

		log.Printf("Applying migration: %s", entry.Name())
		content, err := migrationFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}

		if _, err := s.db.Exec(string(content)); err != nil {
			return fmt.Errorf("migration %s failed: %w", entry.Name(), err)
		}
	}

	return nil
}

func (s *Store) seed() error {
	// Seed Groups
	var groupCount int
	row := s.db.QueryRow("SELECT COUNT(*) FROM groups")
	if err := row.Scan(&groupCount); err != nil {
		return err
	}

	if groupCount == 0 {
		log.Println("Seeding default group...")
		_, err := s.db.Exec("INSERT INTO groups (id, name, icon) VALUES (?, ?, ?)", "g-default", "Default", "Server")
		if err != nil {
			return err
		}
		log.Println("Default group 'Default' (id: g-default) created")
	}

	return nil
}

func (s *Store) Reset() error {
	// Disable FKs to allow dropping tables regardless of order
	if _, err := s.db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		return err
	}

	tables := []string{
		"users", "sessions", "groups", "monitors", "monitor_checks",
		"monitor_events", "status_pages", "api_keys", "settings", "monitor_outages",
		"notification_channels", "incidents",
	}

	for _, table := range tables {
		if _, err := s.db.Exec("DROP TABLE IF EXISTS " + table); err != nil {
			return err
		}
	}

	// Re-enable FKs
	if _, err := s.db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return err
	}

	// Re-run migrations and seeds
	if err := s.migrate(); err != nil {
		return err
	}
	return s.seed()
}
