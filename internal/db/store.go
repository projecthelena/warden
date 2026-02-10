package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

// Database dialect constants
const (
	DialectSQLite   = "sqlite"
	DialectPostgres = "postgres"
)

//go:embed migrations/sqlite/*.sql
var sqliteMigrationFS embed.FS

//go:embed migrations/postgres/*.sql
var postgresMigrationFS embed.FS

// DBConfig holds database configuration
type DBConfig struct {
	Type string // "sqlite" or "postgres"
	Path string // SQLite file path
	URL  string // PostgreSQL connection URL
}

type Store struct {
	db      *sql.DB
	dialect string
}

// NewStore creates a new store with the given configuration.
// For SQLite: pass DBConfig{Type: "sqlite", Path: "path/to/db.sqlite"}
// For PostgreSQL: pass DBConfig{Type: "postgres", URL: "postgres://user:pass@host/db"}
func NewStore(cfg DBConfig) (*Store, error) {
	var db *sql.DB
	var err error
	var dialect string

	switch cfg.Type {
	case DialectPostgres, "postgresql":
		dialect = DialectPostgres
		db, err = sql.Open("postgres", cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to open postgres: %w", err)
		}
	default:
		// Default to SQLite
		dialect = DialectSQLite
		db, err = sql.Open("sqlite3", cfg.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to open sqlite: %w", err)
		}
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// SQLite-specific settings
	if dialect == DialectSQLite {
		// SQLite only supports one writer at a time. Limiting to a single
		// connection also ensures that in-memory databases (:memory:) work
		// correctly with Go's connection pool — each connection would
		// otherwise get its own isolated database.
		db.SetMaxOpenConns(1)
		if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
			return nil, err
		}
	}

	s := &Store{db: db, dialect: dialect}
	if err := s.migrate(); err != nil {
		return nil, err
	}

	if err := s.seed(); err != nil {
		return nil, err
	}

	return s, nil
}

// Dialect returns the database dialect ("sqlite" or "postgres")
func (s *Store) Dialect() string {
	return s.dialect
}

// rebind converts ? placeholders to $1, $2, etc. for PostgreSQL
// SQLite queries pass through unchanged
func (s *Store) rebind(query string) string {
	if s.dialect != DialectPostgres {
		return query
	}
	var result []byte
	placeholder := 1
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			result = append(result, '$')
			result = append(result, []byte(fmt.Sprintf("%d", placeholder))...)
			placeholder++
		} else {
			result = append(result, query[i])
		}
	}
	return string(result)
}

// IsSQLite returns true if using SQLite
func (s *Store) IsSQLite() bool {
	return s.dialect == DialectSQLite
}

// IsPostgres returns true if using PostgreSQL
func (s *Store) IsPostgres() bool {
	return s.dialect == DialectPostgres
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	// Select the appropriate migration filesystem and Goose dialect
	var embedFS embed.FS
	var migrationPath string
	var gooseDialect goose.Dialect

	switch s.dialect {
	case DialectPostgres:
		embedFS = postgresMigrationFS
		migrationPath = "migrations/postgres"
		gooseDialect = goose.DialectPostgres
	default:
		embedFS = sqliteMigrationFS
		migrationPath = "migrations/sqlite"
		gooseDialect = goose.DialectSQLite3
	}

	// Extract the migrations subdirectory from the embedded FS
	migrationsDir, err := fs.Sub(embedFS, migrationPath)
	if err != nil {
		return err
	}

	// Use Provider API which is thread-safe (avoids global state race conditions in tests)
	provider, err := goose.NewProvider(gooseDialect, s.db, migrationsDir)
	if err != nil {
		return err
	}

	log.Println("Running database migrations...")
	if _, err := provider.Up(context.Background()); err != nil {
		return err
	}
	log.Println("Database migrations complete")
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
		var err error
		if s.dialect == DialectPostgres {
			_, err = s.db.Exec("INSERT INTO groups (id, name, icon) VALUES ($1, $2, $3)", "g-default", "Default", "Server")
		} else {
			_, err = s.db.Exec("INSERT INTO groups (id, name, icon) VALUES (?, ?, ?)", "g-default", "Default", "Server")
		}
		if err != nil {
			return err
		}
		log.Println("Default group 'Default' (id: g-default) created")
	}

	return nil
}

// allowedResetTables is a whitelist of table names that can be dropped during reset.
// SECURITY: This prevents potential SQL injection if table names were ever derived from user input.
var allowedResetTables = map[string]bool{
	"users":                 true,
	"sessions":              true,
	"groups":                true,
	"monitors":              true,
	"monitor_checks":        true,
	"monitor_events":        true,
	"status_pages":          true,
	"api_keys":              true,
	"settings":              true,
	"monitor_outages":       true,
	"notification_channels": true,
	"incidents":             true,
	"goose_db_version":      true,
}

// isValidTableName checks if a table name is in the allowed whitelist.
// SECURITY: Defense in depth - validates table names even though they're currently hardcoded.
func isValidTableName(table string) bool {
	return allowedResetTables[table]
}

func (s *Store) Reset() error {
	// Disable FKs to allow dropping tables regardless of order
	if s.dialect == DialectSQLite {
		if _, err := s.db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
			return err
		}
	}

	tables := []string{
		"users", "sessions", "groups", "monitors", "monitor_checks",
		"monitor_events", "status_pages", "api_keys", "settings", "monitor_outages",
		"notification_channels", "incidents",
		"goose_db_version", // Goose migration tracking table
	}

	for _, table := range tables {
		// SECURITY: Validate table name against whitelist before using in query
		if !isValidTableName(table) {
			return fmt.Errorf("invalid table name: %s", table)
		}

		if s.dialect == DialectPostgres {
			// PostgreSQL: use CASCADE to handle foreign key constraints
			if _, err := s.db.Exec("DROP TABLE IF EXISTS " + table + " CASCADE"); err != nil {
				return err
			}
		} else {
			if _, err := s.db.Exec("DROP TABLE IF EXISTS " + table); err != nil {
				return err
			}
		}
	}

	// Re-enable FKs (SQLite only)
	if s.dialect == DialectSQLite {
		if _, err := s.db.Exec("PRAGMA foreign_keys = ON"); err != nil {
			return err
		}
	}

	// Re-run migrations and seeds
	if err := s.migrate(); err != nil {
		return err
	}
	return s.seed()
}

