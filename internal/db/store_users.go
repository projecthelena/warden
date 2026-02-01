package db

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrInvalidPass  = errors.New("invalid password")
)

type User struct {
	ID          int64
	Username    string
	Password    string // Hash
	Timezone    string
	CreatedAt   time.Time
	Email       string
	SSOProvider string
	SSOID       string
}

type Session struct {
	Token     string
	UserID    int64
	ExpiresAt time.Time
}

func (s *Store) Authenticate(username, password string) (*User, error) {
	// username = strings.ToLower(strings.TrimSpace(username)) // REMOVED for Strict Mode
	username = strings.TrimSpace(username) // Only trim valid white space
	var u User
	row := s.db.QueryRow(s.rebind("SELECT id, username, password_hash, created_at, COALESCE(timezone, 'UTC') FROM users WHERE username = ?"), username)
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
	_, err := s.db.Exec(s.rebind("INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)"), token, userID, expiresAt)
	return err
}

func (s *Store) GetSession(token string) (*Session, error) {
	var sess Session
	row := s.db.QueryRow(s.rebind("SELECT token, user_id, expires_at FROM sessions WHERE token = ? AND expires_at > ?"), token, time.Now())
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
	var email, ssoProvider, ssoID sql.NullString
	row := s.db.QueryRow(s.rebind("SELECT id, username, created_at, COALESCE(timezone, 'UTC'), email, sso_provider, sso_id FROM users WHERE id = ?"), id)
	err := row.Scan(&u.ID, &u.Username, &u.CreatedAt, &u.Timezone, &email, &ssoProvider, &ssoID)
	if err != nil {
		return nil, err
	}
	// Redact password
	u.Password = ""
	u.Email = email.String
	u.SSOProvider = ssoProvider.String
	u.SSOID = ssoID.String
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
	username = strings.ToLower(strings.TrimSpace(username))
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	// Using context if we wanted to enforce timeouts, but standard Exec is fine for now
	_, err = s.db.Exec(s.rebind("INSERT INTO users (username, password_hash, timezone) VALUES (?, ?, ?)"), username, string(hash), timezone)
	return err
}

func (s *Store) UpdateUser(id int64, password, timezone string) error {
	if password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		_, err = s.db.Exec(s.rebind("UPDATE users SET password_hash = ?, timezone = ? WHERE id = ?"), string(hash), timezone, id)
		return err
	}
	_, err := s.db.Exec(s.rebind("UPDATE users SET timezone = ? WHERE id = ?"), timezone, id)
	return err
}

func (s *Store) VerifyPassword(userID int64, password string) error {
	var hash string
	err := s.db.QueryRow(s.rebind("SELECT password_hash FROM users WHERE id = ?"), userID).Scan(&hash)
	if err == sql.ErrNoRows {
		return ErrUserNotFound
	}
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return ErrInvalidPass
	}
	return nil
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec(s.rebind("DELETE FROM sessions WHERE token = ?"), token)
	return err
}

// GetUserByEmail retrieves a user by their email address.
func (s *Store) GetUserByEmail(email string) (*User, error) {
	var u User
	var emailVal, ssoProvider, ssoID sql.NullString
	row := s.db.QueryRow(s.rebind("SELECT id, username, created_at, COALESCE(timezone, 'UTC'), email, sso_provider, sso_id FROM users WHERE email = ?"), email)
	err := row.Scan(&u.ID, &u.Username, &u.CreatedAt, &u.Timezone, &emailVal, &ssoProvider, &ssoID)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	u.Email = emailVal.String
	u.SSOProvider = ssoProvider.String
	u.SSOID = ssoID.String
	return &u, nil
}

// FindOrCreateSSOUser finds a user by SSO provider and ID, or creates a new one.
// If a user with the same email exists, it links the SSO credentials to that account.
// If autoProvision is false and no existing user is found, returns ErrUserNotFound.
func (s *Store) FindOrCreateSSOUser(provider, ssoID, email, name string, autoProvision bool) (*User, error) {
	// First, try to find by SSO provider and ID
	var u User
	var emailVal, ssoProvider, ssoIDVal sql.NullString
	row := s.db.QueryRow(s.rebind("SELECT id, username, created_at, COALESCE(timezone, 'UTC'), email, sso_provider, sso_id FROM users WHERE sso_provider = ? AND sso_id = ?"), provider, ssoID)
	err := row.Scan(&u.ID, &u.Username, &u.CreatedAt, &u.Timezone, &emailVal, &ssoProvider, &ssoIDVal)
	if err == nil {
		// Found existing SSO user
		u.Email = emailVal.String
		u.SSOProvider = ssoProvider.String
		u.SSOID = ssoIDVal.String
		return &u, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	// Not found by SSO, try to find by email to link accounts
	existingUser, err := s.GetUserByEmail(email)
	if err == nil {
		// Link SSO to existing account (always allowed - user already exists)
		_, err = s.db.Exec(s.rebind("UPDATE users SET sso_provider = ?, sso_id = ? WHERE id = ?"), provider, ssoID, existingUser.ID)
		if err != nil {
			return nil, err
		}
		existingUser.SSOProvider = provider
		existingUser.SSOID = ssoID
		return existingUser, nil
	}
	if err != ErrUserNotFound {
		return nil, err
	}

	// No existing user found - check if auto-provisioning is allowed
	if !autoProvision {
		return nil, ErrUserNotFound
	}

	// Create new user with SSO credentials (no password needed for SSO-only users)
	username := strings.ToLower(strings.TrimSpace(name))
	if username == "" {
		// Extract username from email
		parts := strings.Split(email, "@")
		if len(parts) > 0 {
			username = strings.ToLower(parts[0])
		}
	}
	// Remove any characters that aren't alphanumeric or underscore
	cleanUsername := ""
	for _, c := range username {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			cleanUsername += string(c)
		}
	}
	username = cleanUsername
	if username == "" {
		username = "user"
	}

	// Make username unique by appending numbers if needed
	baseUsername := username
	counter := 1
	for {
		var exists int
		err = s.db.QueryRow(s.rebind("SELECT COUNT(*) FROM users WHERE username = ?"), username).Scan(&exists)
		if err != nil {
			return nil, err
		}
		if exists == 0 {
			break
		}
		username = baseUsername + strconv.Itoa(counter)
		counter++
	}

	// Insert new user with empty password (SSO-only user)
	var newID int64
	if s.IsPostgres() {
		err = s.db.QueryRow("INSERT INTO users (username, password_hash, email, sso_provider, sso_id) VALUES ($1, '', $2, $3, $4) RETURNING id",
			username, email, provider, ssoID).Scan(&newID)
	} else {
		result, execErr := s.db.Exec("INSERT INTO users (username, password_hash, email, sso_provider, sso_id) VALUES (?, '', ?, ?, ?)",
			username, email, provider, ssoID)
		if execErr != nil {
			return nil, execErr
		}
		newID, err = result.LastInsertId()
	}
	if err != nil {
		return nil, err
	}

	return &User{
		ID:          newID,
		Username:    username,
		Email:       email,
		SSOProvider: provider,
		SSOID:       ssoID,
		Timezone:    "UTC",
	}, nil
}

// Just to avoid unused import error for context if not used
var _ = context.Background
