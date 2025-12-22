package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrInvalidPass  = errors.New("invalid password")
)

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
	// Using context if we wanted to enforce timeouts, but standard Exec is fine for now
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

// Just to avoid unused import error for context if not used
var _ = context.Background
