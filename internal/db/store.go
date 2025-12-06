package db

import (
	"database/sql"
	"errors"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrInvalidPass  = errors.New("invalid password")
)

type Store struct {
	db *sql.DB
}

type User struct {
	ID        int64
	Username  string
	Password  string // Hash
	Timezone  string
	CreatedAt time.Time
}

type Session struct {
	Token     string
	UserID    int64
	ExpiresAt time.Time
}

type Group struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
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
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL,
		expires_at DATETIME NOT NULL,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);
	CREATE TABLE IF NOT EXISTS groups (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS monitors (
		id TEXT PRIMARY KEY,
		group_id TEXT NOT NULL,
		name TEXT NOT NULL,
		url TEXT NOT NULL,
		active BOOLEAN DEFAULT TRUE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(group_id) REFERENCES groups(id) ON DELETE CASCADE
	);
	CREATE TABLE IF NOT EXISTS monitor_checks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		monitor_id TEXT NOT NULL,
		status TEXT NOT NULL,
		latency INTEGER NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(monitor_id) REFERENCES monitors(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_monitor_checks_monitor_id_ts ON monitor_checks(monitor_id, timestamp DESC);

	CREATE TABLE IF NOT EXISTS monitor_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		monitor_id TEXT NOT NULL,
		type TEXT NOT NULL,
		message TEXT NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(monitor_id) REFERENCES monitors(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_monitor_events_monitor_id ON monitor_events(monitor_id);

	CREATE TABLE IF NOT EXISTS status_pages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slug TEXT UNIQUE NOT NULL,
		title TEXT NOT NULL,
		group_id TEXT,
		public BOOLEAN DEFAULT FALSE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(group_id) REFERENCES groups(id) ON DELETE CASCADE
	);
	`
	if _, err := s.db.Exec(query); err != nil {
		return err
	}
	// Add timezone column if missing
	_, _ = s.db.Exec("ALTER TABLE users ADD COLUMN timezone TEXT DEFAULT 'UTC'")

	// Seed default global status page
	// Use INSERT OR IGNORE to prevent overwriting 'public' status of existing page
	_, err := s.db.Exec(`INSERT OR IGNORE INTO status_pages (slug, title, group_id, public) VALUES (?, ?, ?, ?)`, "all", "Global Status", nil, false)
	if err != nil {
		log.Printf("Failed to seed global status page: %v", err)
	}

	return nil
}

// ... seed implementation remains ...

// Group CRUD

func (s *Store) CreateGroup(g Group) error {
	_, err := s.db.Exec("INSERT INTO groups (id, name, created_at) VALUES (?, ?, ?)", g.ID, g.Name, time.Now())
	return err
}

func (s *Store) DeleteGroup(id string) error {
	_, err := s.db.Exec("DELETE FROM groups WHERE id = ?", id)
	return err
}

// Monitor CRUD

type Monitor struct {
	ID        string
	GroupID   string
	Name      string
	URL       string
	Active    bool
	CreatedAt time.Time
}

func (s *Store) CreateMonitor(m Monitor) error {
	_, err := s.db.Exec("INSERT INTO monitors (id, group_id, name, url, active, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		m.ID, m.GroupID, m.Name, m.URL, m.Active, time.Now())
	return err
}

func (s *Store) CreateEvent(monitorID, eventType, message string) error {
	_, err := s.db.Exec("INSERT INTO monitor_events (monitor_id, type, message) VALUES (?, ?, ?)",
		monitorID, eventType, message)
	return err
}

// GetMonitors returns all monitors (optionally filtered by group, but for now all)
func (s *Store) GetMonitors() ([]Monitor, error) {
	rows, err := s.db.Query("SELECT id, group_id, name, url, active, created_at FROM monitors ORDER BY created_at ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var monitors []Monitor
	for rows.Next() {
		var m Monitor
		if err := rows.Scan(&m.ID, &m.GroupID, &m.Name, &m.URL, &m.Active, &m.CreatedAt); err != nil {
			return nil, err
		}
		monitors = append(monitors, m)
	}
	return monitors, nil
}

func (s *Store) DeleteMonitor(id string) error {
	_, err := s.db.Exec("DELETE FROM monitors WHERE id = ?", id)
	return err
}

type CheckResult struct {
	MonitorID string
	Status    string
	Latency   int64
	Timestamp time.Time
}

func (s *Store) BatchInsertChecks(checks []CheckResult) error {
	if len(checks) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT INTO monitor_checks (monitor_id, status, latency, timestamp) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range checks {
		_, err := stmt.Exec(c.MonitorID, c.Status, c.Latency, c.Timestamp)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetMonitorChecks returns the last N checks for a monitor
func (s *Store) GetMonitorChecks(monitorID string, limit int) ([]CheckResult, error) {
	query := `SELECT monitor_id, status, latency, timestamp FROM monitor_checks 
			  WHERE monitor_id = ? ORDER BY timestamp DESC LIMIT ?`

	rows, err := s.db.Query(query, monitorID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []CheckResult
	for rows.Next() {
		var c CheckResult
		if err := rows.Scan(&c.MonitorID, &c.Status, &c.Latency, &c.Timestamp); err != nil {
			return nil, err
		}
		checks = append(checks, c)
	}
	return checks, nil
}

func (s *Store) seed() error {
	// Seed Users
	var userCount int
	row := s.db.QueryRow("SELECT COUNT(*) FROM users")
	if err := row.Scan(&userCount); err != nil {
		return err
	}

	if userCount == 0 {
		log.Println("Creating default admin user...")
		hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
		_, err := s.db.Exec("INSERT INTO users (username, password_hash, timezone) VALUES (?, ?, ?)", "admin", string(hash), "UTC")
		if err != nil {
			return err
		}
		log.Println("Default user created: username='admin', password='password'")
	}

	// Seed Groups
	var groupCount int
	row = s.db.QueryRow("SELECT COUNT(*) FROM groups")
	if err := row.Scan(&groupCount); err != nil {
		return err
	}

	if groupCount == 0 {
		log.Println("Seeding default group...")
		_, err := s.db.Exec("INSERT INTO groups (id, name) VALUES (?, ?)", "default", "Default")
		if err != nil {
			return err
		}
		log.Println("Default group 'Default' (id: default) created")
	}

	// Seed Monitors
	var monitorCount int
	row = s.db.QueryRow("SELECT COUNT(*) FROM monitors")
	if err := row.Scan(&monitorCount); err != nil {
		return err
	}

	if monitorCount == 0 {
		log.Println("Seeding default monitor...")
		_, err := s.db.Exec("INSERT INTO monitors (id, group_id, name, url, active) VALUES (?, ?, ?, ?, ?)",
			"m1", "default", "Example Monitor", "https://google.com", true)
		if err != nil {
			return err
		}
		log.Println("Default monitor created")
	}

	return nil
}

func (s *Store) Authenticate(username, password string) (*User, error) {
	var u User
	row := s.db.QueryRow("SELECT id, username, password_hash, created_at, COALESCE(timezone, 'UTC') FROM users WHERE username = ?", username)
	err := row.Scan(&u.ID, &u.Username, &u.Password, &u.CreatedAt, &u.Timezone)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return nil, ErrInvalidPass
	}

	return &u, nil
}

func (s *Store) CreateSession(userID int64, token string, expiresAt time.Time) error {
	_, err := s.db.Exec("INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)", token, userID, expiresAt)
	return err
}

func (s *Store) GetSession(token string) (*Session, error) {
	var sess Session
	row := s.db.QueryRow("SELECT token, user_id, expires_at FROM sessions WHERE token = ? AND expires_at > ?", token, time.Now())
	err := row.Scan(&sess.Token, &sess.UserID, &sess.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, nil // Not found or expired
	}
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Store) GetUser(id int64) (*User, error) {
	var u User
	row := s.db.QueryRow("SELECT id, username, created_at, COALESCE(timezone, 'UTC') FROM users WHERE id = ?", id)
	err := row.Scan(&u.ID, &u.Username, &u.CreatedAt, &u.Timezone)
	if err != nil {
		return nil, err
	}
	// Redact password
	u.Password = ""
	return &u, nil
}

func (s *Store) UpdateUser(id int64, password, timezone string) error {
	if password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		_, err = s.db.Exec("UPDATE users SET password_hash = ?, timezone = ? WHERE id = ?", string(hash), timezone, id)
		return err
	}
	_, err := s.db.Exec("UPDATE users SET timezone = ? WHERE id = ?", timezone, id)
	return err
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE token = ?", token)
	return err
}

func (s *Store) GetGroups() ([]Group, error) {
	rows, err := s.db.Query("SELECT id, name, created_at FROM groups ORDER BY created_at ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.CreatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, nil
}

// Status Pages

// StatusPage Struct
type StatusPage struct {
	ID        int64     `json:"id"`
	Slug      string    `json:"slug"`
	Title     string    `json:"title"`
	GroupID   *string   `json:"groupId"` // Nullable
	Public    bool      `json:"public"`
	CreatedAt time.Time `json:"createdAt"`
}

// GetStatusPages returns all status page configs
func (s *Store) GetStatusPages() ([]StatusPage, error) {
	rows, err := s.db.Query("SELECT id, slug, title, group_id, public, created_at FROM status_pages")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pages []StatusPage
	for rows.Next() {
		var p StatusPage
		var groupID sql.NullString
		if err := rows.Scan(&p.ID, &p.Slug, &p.Title, &groupID, &p.Public, &p.CreatedAt); err != nil {
			return nil, err
		}
		if groupID.Valid {
			s := groupID.String
			p.GroupID = &s
		}
		pages = append(pages, p)
	}
	return pages, nil
}

// GetStatusPageBySlug returns a specific status page config
func (s *Store) GetStatusPageBySlug(slug string) (*StatusPage, error) {
	var p StatusPage
	var groupID sql.NullString
	err := s.db.QueryRow("SELECT id, slug, title, group_id, public, created_at FROM status_pages WHERE slug = ?", slug).
		Scan(&p.ID, &p.Slug, &p.Title, &groupID, &p.Public, &p.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if groupID.Valid {
		s := groupID.String
		p.GroupID = &s
	}
	return &p, nil
}

// UpsertStatusPage creates or updates a status page config
func (s *Store) UpsertStatusPage(slug, title string, groupID *string, public bool) error {
	_, err := s.db.Exec(`
		INSERT INTO status_pages (slug, title, group_id, public)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(slug) DO UPDATE SET
			title=excluded.title,
			group_id=excluded.group_id,
			public=excluded.public
	`, slug, title, groupID, public)
	return err
}

// ToggleStatusPage toggles the public status
func (s *Store) ToggleStatusPage(slug string, public bool) error {
	_, err := s.db.Exec("UPDATE status_pages SET public = ? WHERE slug = ?", public, slug)
	return err
}
