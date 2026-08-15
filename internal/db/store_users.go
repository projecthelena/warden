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
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidPass        = errors.New("invalid password")
	ErrAccountLinkingNeed = errors.New("account exists with this email, SSO linking requires verification")
)

type User struct {
	ID          int64
	Username    string
	Password    string // #nosec G117 -- stores bcrypt hash, redacted in GetUser()
	Timezone    string
	CreatedAt   time.Time
	Email       string
	SSOProvider string
	SSOID       string
	AvatarURL   string
	DisplayName string
	Role        string
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
	row := s.db.QueryRow(s.rebind("SELECT id, username, password_hash, created_at, COALESCE(timezone, 'UTC'), COALESCE(role, 'admin') FROM users WHERE username = ?"), username)
	err := row.Scan(&u.ID, &u.Username, &u.Password, &u.CreatedAt, &u.Timezone, &u.Role)
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
	var email, ssoProvider, ssoID, avatarURL, displayName sql.NullString
	row := s.db.QueryRow(s.rebind("SELECT id, username, created_at, COALESCE(timezone, 'UTC'), email, sso_provider, sso_id, avatar_url, display_name, COALESCE(role, 'admin') FROM users WHERE id = ?"), id)
	err := row.Scan(&u.ID, &u.Username, &u.CreatedAt, &u.Timezone, &email, &ssoProvider, &ssoID, &avatarURL, &displayName, &u.Role)
	if err != nil {
		return nil, err
	}
	// Redact password
	u.Password = ""
	u.Email = email.String
	u.SSOProvider = ssoProvider.String
	u.SSOID = ssoID.String
	u.AvatarURL = avatarURL.String
	u.DisplayName = displayName.String
	return &u, nil
}

// HasUsers checks if any users exist in the database.
func (s *Store) HasUsers() (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count > 0, err
}

// IsSetupComplete performs an atomic check for setup completion.
// SECURITY: This prevents race conditions where multiple concurrent requests
// could both pass the setup check and create multiple admin users.
func (s *Store) IsSetupComplete() (bool, error) {
	// Single atomic query that checks both conditions
	var isComplete bool
	var query string
	if s.IsPostgres() {
		query = `SELECT (EXISTS(SELECT 1 FROM users) OR EXISTS(SELECT 1 FROM settings WHERE key = 'setup_completed' AND value = 'true'))`
	} else {
		query = `SELECT (EXISTS(SELECT 1 FROM users) OR EXISTS(SELECT 1 FROM settings WHERE key = 'setup_completed' AND value = 'true'))`
	}
	err := s.db.QueryRow(query).Scan(&isComplete)
	return isComplete, err
}

// CreateUser creates a new user with the specified role.
func (s *Store) CreateUser(username, password, timezone, role string) error {
	username = strings.ToLower(strings.TrimSpace(username))
	if role == "" {
		role = "admin"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(s.rebind("INSERT INTO users (username, password_hash, timezone, role) VALUES (?, ?, ?, ?)"), username, string(hash), timezone, role)
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

// ValidatePassword enforces the login password policy: at least 8 characters, one digit
// and one special character. Setup, create-user and the password-reset paths all share it
// so a reset can never set a password the login form would later reject.
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	hasNumber, hasSpecial := false, false
	for _, c := range password {
		switch {
		case c >= '0' && c <= '9':
			hasNumber = true
		case (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9'):
			hasSpecial = true
		}
	}
	if !hasNumber {
		return errors.New("password must contain at least one number")
	}
	if !hasSpecial {
		return errors.New("password must contain at least one special character")
	}
	return nil
}

// SetUserPassword replaces a user's password (admin reset or CLI recovery) and revokes
// their existing sessions, so a reset actually locks out whoever knew the old one.
// Returns ErrUserNotFound if no user has that id.
func (s *Store) SetUserPassword(id int64, password string) error {
	if err := ValidatePassword(password); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(s.rebind("UPDATE users SET password_hash = ? WHERE id = ?"), string(hash), id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrUserNotFound
	}
	// Revoke every session so the old password stops working immediately.
	return s.DeleteUserSessions(id, "")
}

// ResetPasswordByUsername is the CLI recovery path: resolve the username and reset it.
// Returns ErrUserNotFound if the username doesn't exist.
func (s *Store) ResetPasswordByUsername(username, password string) error {
	username = strings.ToLower(strings.TrimSpace(username))
	var id int64
	err := s.db.QueryRow(s.rebind("SELECT id FROM users WHERE username = ?"), username).Scan(&id)
	if err == sql.ErrNoRows {
		return ErrUserNotFound
	}
	if err != nil {
		return err
	}
	return s.SetUserPassword(id, password)
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

// DeleteUserSessions deletes all sessions for a user.
// If exceptToken is non-empty, that session will be preserved (e.g., current session).
func (s *Store) DeleteUserSessions(userID int64, exceptToken string) error {
	if exceptToken != "" {
		_, err := s.db.Exec(s.rebind("DELETE FROM sessions WHERE user_id = ? AND token != ?"), userID, exceptToken)
		return err
	}
	_, err := s.db.Exec(s.rebind("DELETE FROM sessions WHERE user_id = ?"), userID)
	return err
}

// GetUserByEmail retrieves a user by their email address.
func (s *Store) GetUserByEmail(email string) (*User, error) {
	var u User
	var emailVal, ssoProvider, ssoID, avatarURL, displayName sql.NullString
	row := s.db.QueryRow(s.rebind("SELECT id, username, created_at, COALESCE(timezone, 'UTC'), email, sso_provider, sso_id, avatar_url, display_name, COALESCE(role, 'admin') FROM users WHERE email = ?"), email)
	err := row.Scan(&u.ID, &u.Username, &u.CreatedAt, &u.Timezone, &emailVal, &ssoProvider, &ssoID, &avatarURL, &displayName, &u.Role)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	u.Email = emailVal.String
	u.SSOProvider = ssoProvider.String
	u.SSOID = ssoID.String
	u.AvatarURL = avatarURL.String
	u.DisplayName = displayName.String
	return &u, nil
}

// FindOrCreateSSOUser finds a user by SSO provider and ID, or creates a new one.
// If a user with the same email exists, it links the SSO credentials to that account.
// If autoProvision is false and no existing user is found, returns ErrUserNotFound.
// SECURITY: This function uses a transaction to prevent race conditions during account linking.
func (s *Store) FindOrCreateSSOUser(provider, ssoID, email, name, avatarURL string, autoProvision bool) (*User, error) {
	// SECURITY: Use a transaction to prevent race conditions where two concurrent
	// SSO logins could both check for the same email and try to link/create users,
	// potentially leading to duplicate accounts or inconsistent state.
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }() // Rollback is no-op after Commit

	// First, try to find by SSO provider and ID
	var u User
	var emailVal, ssoProvider, ssoIDVal, avatarVal, displayNameVal sql.NullString
	row := tx.QueryRow(s.rebind("SELECT id, username, created_at, COALESCE(timezone, 'UTC'), email, sso_provider, sso_id, avatar_url, display_name, COALESCE(role, 'admin') FROM users WHERE sso_provider = ? AND sso_id = ?"), provider, ssoID)
	err = row.Scan(&u.ID, &u.Username, &u.CreatedAt, &u.Timezone, &emailVal, &ssoProvider, &ssoIDVal, &avatarVal, &displayNameVal, &u.Role)
	if err == nil {
		// Found existing SSO user - update avatar and display_name if changed
		if avatarURL != "" || name != "" {
			_, _ = tx.Exec(s.rebind("UPDATE users SET avatar_url = ?, display_name = ? WHERE id = ?"), avatarURL, name, u.ID)
			avatarVal.String = avatarURL
			displayNameVal.String = name
		}
		u.Email = emailVal.String
		u.SSOProvider = ssoProvider.String
		u.SSOID = ssoIDVal.String
		u.AvatarURL = avatarVal.String
		u.DisplayName = displayNameVal.String
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &u, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	// Not found by SSO, try to find by email (within transaction)
	var existingUser User
	var existingEmailVal, existingSSOProvider, existingSSOID, existingAvatarURL, existingDisplayName sql.NullString
	row = tx.QueryRow(s.rebind("SELECT id, username, created_at, COALESCE(timezone, 'UTC'), email, sso_provider, sso_id, avatar_url, display_name, COALESCE(role, 'admin') FROM users WHERE email = ?"), email)
	err = row.Scan(&existingUser.ID, &existingUser.Username, &existingUser.CreatedAt, &existingUser.Timezone, &existingEmailVal, &existingSSOProvider, &existingSSOID, &existingAvatarURL, &existingDisplayName, &existingUser.Role)
	if err == nil {
		// Found user by email - check if they have a password
		// SECURITY: Do not automatically link SSO to existing accounts with passwords.
		// This prevents account takeover if an attacker controls a Google account
		// with the victim's email address.
		var passwordHash string
		_ = tx.QueryRow(s.rebind("SELECT COALESCE(password_hash, '') FROM users WHERE id = ?"), existingUser.ID).Scan(&passwordHash)
		if passwordHash != "" {
			// Account has a password - require explicit linking through settings
			return nil, ErrAccountLinkingNeed
		}
		// Account is SSO-only (no password) - safe to link new SSO provider
		_, err = tx.Exec(s.rebind("UPDATE users SET sso_provider = ?, sso_id = ?, avatar_url = ?, display_name = ? WHERE id = ?"), provider, ssoID, avatarURL, name, existingUser.ID)
		if err != nil {
			return nil, err
		}
		existingUser.Email = existingEmailVal.String
		existingUser.SSOProvider = provider
		existingUser.SSOID = ssoID
		existingUser.AvatarURL = avatarURL
		existingUser.DisplayName = name
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &existingUser, nil
	}
	if err != sql.ErrNoRows {
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

	// Make username unique by appending numbers if needed (within transaction)
	baseUsername := username
	counter := 1
	for {
		var exists int
		err = tx.QueryRow(s.rebind("SELECT COUNT(*) FROM users WHERE username = ?"), username).Scan(&exists)
		if err != nil {
			return nil, err
		}
		if exists == 0 {
			break
		}
		username = baseUsername + strconv.Itoa(counter)
		counter++
	}

	// Insert new user with empty password (SSO-only user), default role = viewer
	var newID int64
	if s.IsPostgres() {
		err = tx.QueryRow("INSERT INTO users (username, password_hash, email, sso_provider, sso_id, avatar_url, display_name, role) VALUES ($1, '', $2, $3, $4, $5, $6, 'viewer') RETURNING id",
			username, email, provider, ssoID, avatarURL, name).Scan(&newID)
	} else {
		result, execErr := tx.Exec("INSERT INTO users (username, password_hash, email, sso_provider, sso_id, avatar_url, display_name, role) VALUES (?, '', ?, ?, ?, ?, ?, 'viewer')",
			username, email, provider, ssoID, avatarURL, name)
		if execErr != nil {
			return nil, execErr
		}
		newID, err = result.LastInsertId()
	}
	if err != nil {
		return nil, err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &User{
		ID:          newID,
		Username:    username,
		Email:       email,
		SSOProvider: provider,
		SSOID:       ssoID,
		AvatarURL:   avatarURL,
		DisplayName: name,
		Timezone:    "UTC",
		Role:        "viewer",
	}, nil
}

// GetUserRole returns the role for a given user ID.
func (s *Store) GetUserRole(id int64) (string, error) {
	var role string
	err := s.db.QueryRow(s.rebind("SELECT COALESCE(role, 'admin') FROM users WHERE id = ?"), id).Scan(&role)
	if err == sql.ErrNoRows {
		return "", ErrUserNotFound
	}
	return role, err
}

// ListUsers returns all users (passwords redacted).
func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query("SELECT id, username, created_at, COALESCE(timezone, 'UTC'), COALESCE(email, ''), COALESCE(sso_provider, ''), COALESCE(avatar_url, ''), COALESCE(display_name, ''), COALESCE(role, 'admin') FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.CreatedAt, &u.Timezone, &u.Email, &u.SSOProvider, &u.AvatarURL, &u.DisplayName, &u.Role); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

// UpdateUserRole changes a user's role.
func (s *Store) UpdateUserRole(id int64, role string) error {
	_, err := s.db.Exec(s.rebind("UPDATE users SET role = ? WHERE id = ?"), role, id)
	return err
}

// DeleteUser removes a user by ID.
func (s *Store) DeleteUser(id int64) error {
	// Also delete their sessions
	_, _ = s.db.Exec(s.rebind("DELETE FROM sessions WHERE user_id = ?"), id)
	_, err := s.db.Exec(s.rebind("DELETE FROM users WHERE id = ?"), id)
	return err
}

// CountAdmins returns the number of users with the admin role.
func (s *Store) CountAdmins() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count)
	return count, err
}

// GetUserStatusPages returns the status page IDs assigned to a user.
func (s *Store) GetUserStatusPages(userID int64) ([]int64, error) {
	rows, err := s.db.Query(s.rebind("SELECT status_page_id FROM user_status_pages WHERE user_id = ?"), userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// SetUserStatusPages replaces the status page assignments for a user.
func (s *Store) SetUserStatusPages(userID int64, pageIDs []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(s.rebind("DELETE FROM user_status_pages WHERE user_id = ?"), userID)
	if err != nil {
		return err
	}

	for _, pid := range pageIDs {
		_, err = tx.Exec(s.rebind("INSERT INTO user_status_pages (user_id, status_page_id) VALUES (?, ?)"), userID, pid)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Just to avoid unused import error for context if not used
var _ = context.Background
