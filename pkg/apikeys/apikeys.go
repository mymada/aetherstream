package apikeys

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Key represents an API key
type Key struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Prefix    string    `json:"prefix"`
	Hash      string    `json:"-"` // never expose
	Scopes    []string  `json:"scopes"`
	CreatedAt time.Time `json:"created_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
}

// Store manages API keys in SQLite
type Store struct {
	db *sql.DB
}

// NewStore creates an API key store
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Migrate creates the api_keys table
func (s *Store) Migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS api_keys (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	prefix TEXT NOT NULL,
	hash TEXT NOT NULL,
	scopes TEXT NOT NULL DEFAULT '',
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	revoked_at DATETIME,
	last_used DATETIME
);
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(prefix);
`
	_, err := s.db.Exec(schema)
	return err
}

// Create generates a new API key and returns the raw key (shown once)
func (s *Store) Create(name string, scopes []string) (*Key, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", fmt.Errorf("generate key: %w", err)
	}
	rawKey := "ak_" + hex.EncodeToString(raw)
	prefix := rawKey[:7] // "ak_" + 4 chars
	hash := hashKey(rawKey)

	id := uuid.New().String()
	scopesStr := strings.Join(scopes, ",")

	_, err := s.db.Exec(
		"INSERT INTO api_keys(id, name, prefix, hash, scopes) VALUES (?, ?, ?, ?, ?)",
		id, name, prefix, hash, scopesStr,
	)
	if err != nil {
		return nil, "", fmt.Errorf("insert key: %w", err)
	}

	return &Key{
		ID:     id,
		Name:   name,
		Prefix: prefix,
		Hash:   hash,
		Scopes: scopes,
	}, rawKey, nil
}

// Revoke marks a key as revoked
func (s *Store) Revoke(id string) error {
	_, err := s.db.Exec(
		"UPDATE api_keys SET revoked_at = CURRENT_TIMESTAMP WHERE id = ? AND revoked_at IS NULL",
		id,
	)
	return err
}

// Validate checks if a raw API key is valid and returns the associated key record
func (s *Store) Validate(rawKey string) (*Key, error) {
	if !strings.HasPrefix(rawKey, "ak_") {
		return nil, fmt.Errorf("invalid key format")
	}
	prefix := rawKey[:7]
	row := s.db.QueryRow(
		"SELECT id, name, prefix, hash, scopes, created_at, revoked_at, last_used FROM api_keys WHERE prefix = ?",
		prefix,
	)
	var k Key
	var scopesStr string
	var revokedAt, lastUsed sql.NullTime
	err := row.Scan(&k.ID, &k.Name, &k.Prefix, &k.Hash, &scopesStr, &k.CreatedAt, &revokedAt, &lastUsed)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("key not found")
	}
	if err != nil {
		return nil, err
	}
	if revokedAt.Valid {
		k.RevokedAt = &revokedAt.Time
	}
	if lastUsed.Valid {
		k.LastUsed = &lastUsed.Time
	}
	if k.RevokedAt != nil {
		return nil, fmt.Errorf("key revoked")
	}
	if k.Hash != hashKey(rawKey) {
		return nil, fmt.Errorf("invalid key")
	}
	if scopesStr != "" {
		k.Scopes = strings.Split(scopesStr, ",")
	}
	// Update last_used
	_, _ = s.db.Exec("UPDATE api_keys SET last_used = CURRENT_TIMESTAMP WHERE id = ?", k.ID)
	return &k, nil
}

// List returns all API keys (without hashes)
func (s *Store) List() ([]Key, error) {
	rows, err := s.db.Query(
		"SELECT id, name, prefix, scopes, created_at, revoked_at, last_used FROM api_keys ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []Key
	for rows.Next() {
		var k Key
		var scopesStr string
		var revokedAt, lastUsed sql.NullTime
		if err := rows.Scan(&k.ID, &k.Name, &k.Prefix, &scopesStr, &k.CreatedAt, &revokedAt, &lastUsed); err != nil {
			continue
		}
		if revokedAt.Valid {
			k.RevokedAt = &revokedAt.Time
		}
		if lastUsed.Valid {
			k.LastUsed = &lastUsed.Time
		}
		if scopesStr != "" {
			k.Scopes = strings.Split(scopesStr, ",")
		}
		keys = append(keys, k)
	}
	return keys, nil
}

// HasScope checks if a key has a specific scope
func HasScope(key *Key, scope string) bool {
	for _, s := range key.Scopes {
		if s == scope || s == "*" {
			return true
		}
	}
	return false
}

func hashKey(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
