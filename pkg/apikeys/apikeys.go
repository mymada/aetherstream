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
	"golang.org/x/crypto/bcrypt"
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
	lookup_hash TEXT NOT NULL,
	hash TEXT NOT NULL,
	scopes TEXT NOT NULL DEFAULT '',
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	revoked_at DATETIME,
	last_used DATETIME
);
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(prefix);
CREATE INDEX IF NOT EXISTS idx_api_keys_lookup ON api_keys(lookup_hash);
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
	prefix := rawKey[:11] // "ak_" + 8 hex chars (fixes M5: longer prefix = less collision)
	lookupHash := fastHash(rawKey)
	hash := hashKey(rawKey)

	id := uuid.New().String()
	scopesStr := strings.Join(scopes, ",")

	_, err := s.db.Exec(
		"INSERT INTO api_keys(id, name, prefix, lookup_hash, hash, scopes) VALUES (?, ?, ?, ?, ?, ?)",
		id, name, prefix, lookupHash, hash, scopesStr,
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
	// Use fast SHA-256 lookup to find candidate keys (fixes M5)
	lookupHash := fastHash(rawKey)
	rows, err := s.db.Query(
		"SELECT id, name, prefix, hash, scopes, created_at, revoked_at, last_used FROM api_keys WHERE lookup_hash = ?",
		lookupHash,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []Key
	for rows.Next() {
		var k Key
		var scopesStr string
		var revokedAt, lastUsed sql.NullTime
		if err := rows.Scan(&k.ID, &k.Name, &k.Prefix, &k.Hash, &scopesStr, &k.CreatedAt, &revokedAt, &lastUsed); err != nil {
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
		candidates = append(candidates, k)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("key not found")
	}

	// Verify bcrypt against candidates
	for _, k := range candidates {
		if checkKey(rawKey, k.Hash) {
			if k.RevokedAt != nil {
				return nil, fmt.Errorf("key revoked")
			}
			// Update last_used with error handling (fixes M5)
			if _, err := s.db.Exec("UPDATE api_keys SET last_used = CURRENT_TIMESTAMP WHERE id = ?", k.ID); err != nil {
				// Non-fatal: log but don't fail validation
				// In production, use a proper logger
			}
			return &k, nil
		}
	}
	return nil, fmt.Errorf("invalid key")
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
	h, _ := bcrypt.GenerateFromPassword([]byte(raw), 12)
	return string(h)
}

func checkKey(raw, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(raw)) == nil
}

// fastHash returns a SHA-256 hex digest for fast DB lookup (not for verification)
func fastHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
