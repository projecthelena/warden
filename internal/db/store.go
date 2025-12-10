package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
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
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Monitors  []Monitor `json:"monitors"`
	CreatedAt time.Time `json:"createdAt"`
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
		icon TEXT DEFAULT 'Server',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS monitors (
		id TEXT PRIMARY KEY,
		group_id TEXT NOT NULL,
		name TEXT NOT NULL,
		url TEXT NOT NULL,
		active BOOLEAN DEFAULT TRUE,
		interval_seconds INTEGER DEFAULT 60,
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

	CREATE TABLE IF NOT EXISTS api_keys (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key_prefix TEXT NOT NULL,
		key_hash TEXT UNIQUE NOT NULL,
		name TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_used_at DATETIME
	);
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT
	);
	INSERT OR IGNORE INTO settings (key, value) VALUES ('latency_threshold', '1000');
	CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(key_prefix);
	`
	if _, err := s.db.Exec(query); err != nil {
		return err
	}
	// Add timezone column if missing
	_, _ = s.db.Exec("ALTER TABLE users ADD COLUMN timezone TEXT DEFAULT 'UTC'")
	// Add status_code column if missing (Migration for rich history)
	_, _ = s.db.Exec("ALTER TABLE monitor_checks ADD COLUMN status_code INTEGER DEFAULT 0")
	// Add interval_seconds column if missing
	_, _ = s.db.Exec("ALTER TABLE monitors ADD COLUMN interval_seconds INTEGER DEFAULT 60")

	// Seed default global status page
	// Use INSERT OR IGNORE to prevent overwriting 'public' status of existing page
	_, err := s.db.Exec(`INSERT OR IGNORE INTO status_pages (slug, title, group_id, public) VALUES (?, ?, ?, ?)`, "all", "Global Status", nil, false)
	if err != nil {
		log.Printf("Failed to seed global status page: %v", err)
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
		"monitor_events", "status_pages", "api_keys", "settings",
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

// Group CRUD

func (s *Store) CreateGroup(g Group) error {
	_, err := s.db.Exec("INSERT INTO groups (id, name, created_at) VALUES (?, ?, ?)", g.ID, g.Name, time.Now())
	return err
}

func (s *Store) DeleteGroup(id string) error {
	_, err := s.db.Exec("DELETE FROM groups WHERE id = ?", id)
	return err
}

func (s *Store) UpdateGroup(id, name string) error {
	_, err := s.db.Exec("UPDATE groups SET name = ? WHERE id = ?", name, id)
	return err
}

// Monitor CRUD

type Monitor struct {
	ID        string    `json:"id"`
	GroupID   string    `json:"groupId"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Active    bool      `json:"active"`
	Interval  int       `json:"interval"` // Seconds
	CreatedAt time.Time `json:"createdAt"`
}

func (s *Store) CreateMonitor(m Monitor) error {
	if m.Interval < 1 {
		m.Interval = 60 // Default safety
	}
	_, err := s.db.Exec("INSERT INTO monitors (id, group_id, name, url, active, interval_seconds, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		m.ID, m.GroupID, m.Name, m.URL, m.Active, m.Interval, time.Now())
	return err
}

func (s *Store) CreateEvent(monitorID, eventType, message string) error {
	_, err := s.db.Exec("INSERT INTO monitor_events (monitor_id, type, message) VALUES (?, ?, ?)",
		monitorID, eventType, message)
	return err
}

// GetMonitors returns all monitors
func (s *Store) GetMonitors() ([]Monitor, error) {
	rows, err := s.db.Query("SELECT id, group_id, name, url, active, interval_seconds, created_at FROM monitors ORDER BY created_at ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var monitors []Monitor
	for rows.Next() {
		var m Monitor
		if err := rows.Scan(&m.ID, &m.GroupID, &m.Name, &m.URL, &m.Active, &m.Interval, &m.CreatedAt); err != nil {
			return nil, err
		}
		monitors = append(monitors, m)
	}
	return monitors, nil
}

func (s *Store) UpdateMonitor(id, name, url string, interval int) error {
	if interval < 1 {
		interval = 60
	}
	res, err := s.db.Exec("UPDATE monitors SET name = ?, url = ?, interval_seconds = ?, active = ? WHERE id = ?", name, url, interval, true, id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("monitor not found")
	}
	return nil
}

func (s *Store) DeleteMonitor(id string) error {
	_, err := s.db.Exec("DELETE FROM monitors WHERE id = ?", id)
	return err
}

type CheckResult struct {
	MonitorID  string    `json:"monitorId"`
	Status     string    `json:"status"`
	Latency    int64     `json:"latency"`
	Timestamp  time.Time `json:"timestamp"`
	StatusCode int       `json:"statusCode"`
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

	stmt, err := tx.Prepare("INSERT INTO monitor_checks (monitor_id, status, latency, timestamp, status_code) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range checks {
		_, err := stmt.Exec(c.MonitorID, c.Status, c.Latency, c.Timestamp, c.StatusCode)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetMonitorChecks returns the last N checks for a monitor
func (s *Store) GetMonitorChecks(monitorID string, limit int) ([]CheckResult, error) {
	query := `SELECT monitor_id, status, latency, timestamp, COALESCE(status_code, 0) FROM monitor_checks 
			  WHERE monitor_id = ? ORDER BY timestamp DESC LIMIT ?`

	rows, err := s.db.Query(query, monitorID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []CheckResult
	for rows.Next() {
		var c CheckResult
		if err := rows.Scan(&c.MonitorID, &c.Status, &c.Latency, &c.Timestamp, &c.StatusCode); err != nil {
			return nil, err
		}
		checks = append(checks, c)
	}
	return checks, nil
}

func (s *Store) PruneMonitorChecks(days int) error {
	_, err := s.db.Exec("DELETE FROM monitor_checks WHERE timestamp < datetime('now', '-' || ? || ' days')", days)
	return err
}

func (s *Store) GetUptimeStats(monitorID string) (float64, float64, float64, error) {
	query := `
		SELECT 
			COUNT(CASE WHEN timestamp > datetime('now', '-1 days') THEN 1 END) as total_24h,
			COUNT(CASE WHEN timestamp > datetime('now', '-1 days') AND status = 'up' THEN 1 END) as up_24h,
			COUNT(CASE WHEN timestamp > datetime('now', '-7 days') THEN 1 END) as total_7d,
			COUNT(CASE WHEN timestamp > datetime('now', '-7 days') AND status = 'up' THEN 1 END) as up_7d,
			COUNT(CASE WHEN timestamp > datetime('now', '-30 days') THEN 1 END) as total_30d,
			COUNT(CASE WHEN timestamp > datetime('now', '-30 days') AND status = 'up' THEN 1 END) as up_30d
		FROM monitor_checks
		WHERE monitor_id = ?
	`
	var t24, u24, t7, u7, t30, u30 int
	err := s.db.QueryRow(query, monitorID).Scan(&t24, &u24, &t7, &u7, &t30, &u30)
	if err != nil {
		return 0, 0, 0, err
	}

	calc := func(up, total int) float64 {
		if total == 0 {
			return 100.0 // Assume 100% if no data? Or 0? Usually 100 or null. Let's return 100 for "No Downtime Recorded" vibe.
		}
		return (float64(up) / float64(total)) * 100.0

	}

	return calc(u24, t24), calc(u7, t7), calc(u30, t30), nil
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
		_, err := s.db.Exec("INSERT INTO groups (id, name, icon) VALUES (?, ?, ?)", "g-default", "Default", "Server")
		if err != nil {
			return err
		}
		log.Println("Default group 'Default' (id: g-default) created")
	}

	// Seed Monitors
	var monitorCount int
	row = s.db.QueryRow("SELECT COUNT(*) FROM monitors")
	if err := row.Scan(&monitorCount); err != nil {
		return err
	}

	if monitorCount == 0 {
		log.Println("Seeding default monitor...")
		_, err := s.db.Exec("INSERT INTO monitors (id, group_id, name, url, active, interval_seconds) VALUES (?, ?, ?, ?, ?, ?)",
			"m-example-monitor-default", "g-default", "Example Monitor", "https://google.com", true, 60)
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
	groupMap := make(map[string]*Group)
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.CreatedAt); err != nil {
			return nil, err
		}
		g.Monitors = []Monitor{} // Initialize empty
		groups = append(groups, g)
	}

	// Create map for easy assignment (pointers to slice elements are tricky if slice reallocates)
	// Better: use index map
	dbGroups := groups
	for i := range dbGroups {
		groupMap[dbGroups[i].ID] = &dbGroups[i]
	}

	// Fetch Monitors
	mRows, err := s.db.Query("SELECT id, group_id, name, url, active, interval_seconds, created_at FROM monitors ORDER BY created_at ASC")
	if err != nil {
		return nil, err // Return collected groups? Or error? Error is safer.
	}
	defer mRows.Close()

	for mRows.Next() {
		var m Monitor
		if err := mRows.Scan(&m.ID, &m.GroupID, &m.Name, &m.URL, &m.Active, &m.Interval, &m.CreatedAt); err != nil {
			return nil, err
		}
		if g, exists := groupMap[m.GroupID]; exists {
			g.Monitors = append(g.Monitors, m)
		}
	}

	return dbGroups, nil
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

// API Keys

type APIKey struct {
	ID        int64      `json:"id"`
	KeyPrefix string     `json:"keyPrefix"`
	Name      string     `json:"name"`
	CreatedAt time.Time  `json:"createdAt"`
	LastUsed  *time.Time `json:"lastUsed,omitempty"`
}

func (s *Store) CreateAPIKey(name string) (string, error) {
	// Generate random key
	keyBytes := make([]byte, 24)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", err
	}
	rawKey := "sk_live_" + hex.EncodeToString(keyBytes)
	prefix := rawKey[:12] // "sk_live_" + first 4 hex chars = 8+4 = 12? No sk_live_ is 8 chars. + 4 = 12. Correct.

	// Hash key
	hash, err := bcrypt.GenerateFromPassword([]byte(rawKey), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	_, err = s.db.Exec("INSERT INTO api_keys (key_prefix, key_hash, name) VALUES (?, ?, ?)",
		prefix, string(hash), name)
	if err != nil {
		return "", err
	}

	return rawKey, nil
}

func (s *Store) ListAPIKeys() ([]APIKey, error) {
	rows, err := s.db.Query("SELECT id, key_prefix, name, created_at, last_used_at FROM api_keys ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var k APIKey
		var lastUsed sql.NullTime
		if err := rows.Scan(&k.ID, &k.KeyPrefix, &k.Name, &k.CreatedAt, &lastUsed); err != nil {
			return nil, err
		}
		if lastUsed.Valid {
			k.LastUsed = &lastUsed.Time
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func (s *Store) DeleteAPIKey(id int64) error {
	_, err := s.db.Exec("DELETE FROM api_keys WHERE id = ?", id)
	return err
}

// Settings methods

func (s *Store) GetSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec("INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", key, value)
	return err
}

func (s *Store) ValidateAPIKey(key string) (bool, error) {
	if len(key) < 12 {
		return false, nil
	}
	prefix := key[:12]

	// Find candidates by prefix
	rows, err := s.db.Query("SELECT id, key_hash FROM api_keys WHERE key_prefix = ?", prefix)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var hash string
		if err := rows.Scan(&id, &hash); err != nil {
			continue
		}

		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(key)); err == nil {
			// update last used async
			go func(keyId int64) {
				// Create a new generic db execution context or ignore error
				// Since we are inside store method, s.db is safe to use concurrently? sql.DB is threadsafe.
				_, _ = s.db.Exec("UPDATE api_keys SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?", keyId)
			}(id)
			return true, nil
		}
	}

	return false, nil
}

type MonitorEvent struct {
	ID        int       `json:"id"`
	MonitorID string    `json:"monitorId"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

func (s *Store) GetMonitorEvents(monitorID string, limit int) ([]MonitorEvent, error) {
	query := `SELECT id, monitor_id, type, message, timestamp FROM monitor_events 
	          WHERE monitor_id = ? ORDER BY timestamp DESC LIMIT ?`

	rows, err := s.db.Query(query, monitorID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []MonitorEvent
	for rows.Next() {
		var e MonitorEvent
		if err := rows.Scan(&e.ID, &e.MonitorID, &e.Type, &e.Message, &e.Timestamp); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}
