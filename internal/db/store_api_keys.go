package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"

	"golang.org/x/crypto/bcrypt"
)

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
