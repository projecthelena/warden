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

type Incident struct {
	ID             string     `json:"id"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Type           string     `json:"type"`     // incident | maintenance
	Severity       string     `json:"severity"` // minor | major | critical
	Status         string     `json:"status"`   // investigation | identified | ... | scheduled | in_progress | completed
	StartTime      time.Time  `json:"startTime"`
	EndTime        *time.Time `json:"endTime,omitempty"`
	AffectedGroups string     `json:"affectedGroups"` // JSON array
	CreatedAt      time.Time  `json:"createdAt"`
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

	CREATE TABLE IF NOT EXISTS monitor_outages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		monitor_id TEXT NOT NULL,
		type TEXT NOT NULL,
		summary TEXT,
		start_time DATETIME DEFAULT CURRENT_TIMESTAMP,
		end_time DATETIME,
		FOREIGN KEY(monitor_id) REFERENCES monitors(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_monitor_outages_monitor_id ON monitor_outages(monitor_id);

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
	CREATE TABLE IF NOT EXISTS incidents (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		description TEXT,
		type TEXT NOT NULL,
		severity TEXT NOT NULL,
		status TEXT NOT NULL,
		start_time DATETIME NOT NULL,
		end_time DATETIME,
		affected_groups TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	INSERT OR IGNORE INTO settings (key, value) VALUES ('latency_threshold', '1000');
	CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(key_prefix);
	
	CREATE TABLE IF NOT EXISTS notification_channels (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		name TEXT NOT NULL,
		config TEXT NOT NULL, -- JSON
		enabled BOOLEAN DEFAULT TRUE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
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
		"monitor_events", "status_pages", "api_keys", "settings", "monitor_outages",
		"notification_channels",
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

// Monitor Outages

type MonitorOutage struct {
	ID          int64      `json:"id"`
	MonitorID   string     `json:"monitorId"`
	Type        string     `json:"type"`
	Summary     string     `json:"summary"`
	StartTime   time.Time  `json:"startTime"`
	EndTime     *time.Time `json:"endTime"`
	MonitorName string     `json:"monitorName"` // Joined
	GroupName   string     `json:"groupName"`   // Joined
	GroupID     string     `json:"groupId"`     // Joined
}

func (s *Store) CreateOutage(monitorID, eventType, summary string) error {
	_, err := s.db.Exec("INSERT INTO monitor_outages (monitor_id, type, summary) VALUES (?, ?, ?)",
		monitorID, eventType, summary)
	return err
}

func (s *Store) CloseOutage(monitorID string) error {
	// Close any active outages for this monitor
	_, err := s.db.Exec("UPDATE monitor_outages SET end_time = CURRENT_TIMESTAMP WHERE monitor_id = ? AND end_time IS NULL", monitorID)
	return err
}

func (s *Store) GetActiveOutages() ([]MonitorOutage, error) {
	query := `
		SELECT o.id, o.monitor_id, o.type, o.summary, o.start_time, m.name, g.name, g.id
		FROM monitor_outages o
		JOIN monitors m ON o.monitor_id = m.id
		JOIN groups g ON m.group_id = g.id
		WHERE o.end_time IS NULL
		ORDER BY o.start_time DESC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var outages []MonitorOutage
	for rows.Next() {
		var o MonitorOutage
		if err := rows.Scan(&o.ID, &o.MonitorID, &o.Type, &o.Summary, &o.StartTime, &o.MonitorName, &o.GroupName, &o.GroupID); err != nil {
			return nil, err
		}
		outages = append(outages, o)
	}
	return outages, nil
}

func (s *Store) GetResolvedOutages(since time.Time) ([]MonitorOutage, error) {
	query := `
		SELECT o.id, o.monitor_id, o.type, o.summary, o.start_time, o.end_time, m.name, g.name, g.id
		FROM monitor_outages o
		JOIN monitors m ON o.monitor_id = m.id
		JOIN groups g ON m.group_id = g.id
		WHERE o.end_time IS NOT NULL AND o.end_time >= ?
		ORDER BY o.end_time DESC
	`
	rows, err := s.db.Query(query, since)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var outages []MonitorOutage
	for rows.Next() {
		var o MonitorOutage
		var endTime sql.NullTime
		if err := rows.Scan(&o.ID, &o.MonitorID, &o.Type, &o.Summary, &o.StartTime, &endTime, &o.MonitorName, &o.GroupName, &o.GroupID); err != nil {
			return nil, err
		}
		if endTime.Valid {
			o.EndTime = &endTime.Time
		}
		outages = append(outages, o)
	}
	return outages, nil
}

// GetMonitors returns all monitors
func (s *Store) GetMonitors() ([]Monitor, error) {
	rows, err := s.db.Query("SELECT id, group_id, name, url, active, interval_seconds, created_at FROM monitors ORDER BY created_at ASC")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

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
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare("INSERT INTO monitor_checks (monitor_id, status, latency, timestamp, status_code) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

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
	defer func() { _ = rows.Close() }()

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
	// Seed Users - REMOVED. handled by setup flow.
	// Logic moved to API check.

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

	// Seed Monitors - REMOVED.
	// We no longer create "Example Monitor" by default in the store.
	// The Setup Wizard creates specific default monitors (Google, GitHub, Cloudflare).

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

// HasUsers checks if any users exist in the database.
func (s *Store) HasUsers() (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count > 0, err
}

// CreateUser creates a new user.
func (s *Store) CreateUser(username, password, timezone string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("INSERT INTO users (username, password_hash, timezone) VALUES (?, ?, ?)", username, string(hash), timezone)
	return err
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
	rows, err := s.db.Query("SELECT id, name, created_at FROM groups ORDER BY name COLLATE NOCASE ASC")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

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
	defer func() { _ = mRows.Close() }()

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
	defer func() { _ = rows.Close() }()

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
	defer func() { _ = rows.Close() }()

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

// Notification Channels

type NotificationChannel struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	Config    string    `json:"config"` // JSON string
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *Store) CreateNotificationChannel(c NotificationChannel) error {
	_, err := s.db.Exec("INSERT INTO notification_channels (id, type, name, config, enabled, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		c.ID, c.Type, c.Name, c.Config, c.Enabled, time.Now())
	return err
}

func (s *Store) GetNotificationChannels() ([]NotificationChannel, error) {
	rows, err := s.db.Query("SELECT id, type, name, config, enabled, created_at FROM notification_channels ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var channels []NotificationChannel
	for rows.Next() {
		var c NotificationChannel
		if err := rows.Scan(&c.ID, &c.Type, &c.Name, &c.Config, &c.Enabled, &c.CreatedAt); err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return channels, nil
}

func (s *Store) DeleteNotificationChannel(id string) error {
	_, err := s.db.Exec("DELETE FROM notification_channels WHERE id = ?", id)
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
	defer func() { _ = rows.Close() }()

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
	defer func() { _ = rows.Close() }()

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

type SystemEvent struct {
	ID          int64     `json:"id"`
	MonitorID   string    `json:"monitorId"`
	MonitorName string    `json:"monitorName"`
	Type        string    `json:"type"` // up, down, degraded
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
}

// GetSystemEvents returns all events for all monitors
func (s *Store) GetSystemEvents(limit int) ([]SystemEvent, error) {
	query := `
		SELECT e.id, e.monitor_id, m.name, e.type, e.message, e.timestamp
		FROM monitor_events e
		JOIN monitors m ON e.monitor_id = m.id
		ORDER BY e.timestamp ASC
	`
	// Note: Fetching ASC to process timeline easily in code, or DESC for display?
	// User needs logic to determine "Active" vs "Closed". ASC is better for state reconstruction.
	// But let's verify if we want to limit?
	// If we limit, we might miss the "start" of an incident.
	// For now, let's fetch all (or a large safety limit) to build accurate history.
	// If performance becomes an issue, we optimize later.

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var events []SystemEvent
	for rows.Next() {
		var e SystemEvent
		if err := rows.Scan(&e.ID, &e.MonitorID, &e.MonitorName, &e.Type, &e.Message, &e.Timestamp); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

type LatencyPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Latency   int64     `json:"latency"`
	Failed    bool      `json:"failed"`
}

func (s *Store) GetLatencyStats(monitorID string, hours int) ([]LatencyPoint, error) {
	var query string
	var groupBy string

	if hours <= 1 {
		// 1h -> Group by Minute
		groupBy = "strftime('%Y-%m-%d %H:%M:00', timestamp)"
	} else if hours <= 168 {
		// 24h & 7d -> Group by Hour
		groupBy = "strftime('%Y-%m-%d %H:00:00', timestamp)"
	} else {
		// 30d+ -> Group by Day
		groupBy = "strftime('%Y-%m-%d', timestamp)"
	}

	query = fmt.Sprintf(`
		SELECT 
			%s as ts_group,
			CAST(AVG(latency) AS INTEGER) as avg_latency,
			MAX(CASE WHEN status != 'up' THEN 1 ELSE 0 END) as failed
		FROM monitor_checks 
		WHERE monitor_id = ? 
		AND datetime(timestamp) > datetime('now', '-' || ? || ' hours')
		GROUP BY ts_group
		ORDER BY ts_group ASC
	`, groupBy)

	rows, err := s.db.Query(query, monitorID, hours)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var points []LatencyPoint
	for rows.Next() {
		var p LatencyPoint
		var tsStr string
		if err := rows.Scan(&tsStr, &p.Latency, &p.Failed); err != nil {
			return nil, err
		}

		// Parse string depending on group?
		// SQLite returns strict formats we requested.
		// If groupBy returns like "2025-12-10 03:00:00"
		if len(tsStr) == 10 { // YYYY-MM-DD
			p.Timestamp, _ = time.Parse("2006-01-02", tsStr)
		} else {
			p.Timestamp, _ = time.Parse("2006-01-02 15:04:05", tsStr)
		}
		points = append(points, p)
	}
	return points, nil
}

// Incident CRUD

func (s *Store) CreateIncident(i Incident) error {
	_, err := s.db.Exec(`
		INSERT INTO incidents (id, title, description, type, severity, status, start_time, end_time, affected_groups, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, i.ID, i.Title, i.Description, i.Type, i.Severity, i.Status, i.StartTime, i.EndTime, i.AffectedGroups, time.Now())
	return err
}

func (s *Store) GetIncidents(since time.Time) ([]Incident, error) {
	query := `
		SELECT id, title, description, type, severity, status, start_time, end_time, affected_groups, created_at 
		FROM incidents 
		WHERE (status != 'resolved' AND status != 'completed') 
		OR start_time >= ? 
		ORDER BY created_at DESC
	`
	rows, err := s.db.Query(query, since)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var incidents []Incident
	for rows.Next() {
		var i Incident
		var endTime sql.NullTime
		if err := rows.Scan(&i.ID, &i.Title, &i.Description, &i.Type, &i.Severity, &i.Status, &i.StartTime, &endTime, &i.AffectedGroups, &i.CreatedAt); err != nil {
			return nil, err
		}
		if endTime.Valid {
			i.EndTime = &endTime.Time
		}
		incidents = append(incidents, i)
	}
	return incidents, nil
}

func (s *Store) UpdateIncident(i Incident) error {
	_, err := s.db.Exec(`
		UPDATE incidents 
		SET title=?, description=?, type=?, severity=?, status=?, start_time=?, end_time=?, affected_groups=?
		WHERE id=?
	`, i.Title, i.Description, i.Type, i.Severity, i.Status, i.StartTime, i.EndTime, i.AffectedGroups, i.ID)
	return err
}

func (s *Store) DeleteIncident(id string) error {
	_, err := s.db.Exec("DELETE FROM incidents WHERE id = ?", id)
	return err
}

// System Stats

type SystemStats struct {
	TotalMonitors    int `json:"totalMonitors"`
	ActiveMonitors   int `json:"activeMonitors"`
	DownMonitors     int `json:"downMonitors"`
	DegradedMonitors int `json:"degradedMonitors"`
	TotalGroups      int `json:"totalGroups"`
	DailyPings       int `json:"dailyPingsEstimate"`
}

func (s *Store) GetSystemStats() (*SystemStats, error) {
	stats := &SystemStats{}

	// Monitor Counts
	if err := s.db.QueryRow("SELECT COUNT(*) FROM monitors").Scan(&stats.TotalMonitors); err != nil {
		log.Printf("Failed to scan total monitors: %v", err)
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM monitors WHERE active = 1").Scan(&stats.ActiveMonitors); err != nil {
		log.Printf("Failed to scan active monitors: %v", err)
	}

	// We don't track status directly in DB "monitors" table except via last check?
	// Wait, Monitor struct has "Status" but it's not in the CREATE TABLE usually?
	// Let's check db schema in migrate().
	// create table monitors: id, group_id, name, url, active, interval_seconds, created_at.
	// Status is transient or computed. BUT we have `monitor_outages` for down/degraded.
	// So "Down" means active outage of type "down".
	if err := s.db.QueryRow("SELECT COUNT(DISTINCT monitor_id) FROM monitor_outages WHERE end_time IS NULL AND type = 'down'").Scan(&stats.DownMonitors); err != nil {
		log.Printf("Failed to scan down monitors: %v", err)
	}
	if err := s.db.QueryRow("SELECT COUNT(DISTINCT monitor_id) FROM monitor_outages WHERE end_time IS NULL AND type = 'degraded'").Scan(&stats.DegradedMonitors); err != nil {
		log.Printf("Failed to scan degraded monitors: %v", err)
	}

	// Groups
	if err := s.db.QueryRow("SELECT COUNT(*) FROM groups").Scan(&stats.TotalGroups); err != nil {
		log.Printf("Failed to scan groups: %v", err)
	}

	// Daily Pings Estimate: SUM( 86400 / interval_seconds ) for active monitors
	// interval_seconds defaults to 60.
	if err := s.db.QueryRow("SELECT COALESCE(SUM(86400 / interval_seconds), 0) FROM monitors WHERE active = 1").Scan(&stats.DailyPings); err != nil {
		log.Printf("Failed to scan daily pings: %v", err)
	}

	return stats, nil
}

func (s *Store) GetDBSize() (int64, error) {
	// PRAGMA page_count * PRAGMA page_size
	var pageCount int64
	var pageSize int64
	if err := s.db.QueryRow("PRAGMA page_count").Scan(&pageCount); err != nil {
		return 0, err
	}
	if err := s.db.QueryRow("PRAGMA page_size").Scan(&pageSize); err != nil {
		return 0, err
	}
	return pageCount * pageSize, nil
}
