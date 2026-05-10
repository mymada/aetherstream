package apikeys

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestStore(t *testing.T) *Store {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	s := NewStore(db)
	require.NoError(t, s.Migrate())
	return s
}

func TestGenerateAPIKey(t *testing.T) {
	s := setupTestStore(t)

	key, raw, err := s.Create("test-key", []string{"read", "write"})
	require.NoError(t, err)
	assert.NotEmpty(t, raw)
	assert.True(t, strings.HasPrefix(raw, "ak_"))
	assert.NotEmpty(t, key.ID)
	assert.Equal(t, "test-key", key.Name)
	assert.Equal(t, raw[:7], key.Prefix)
	assert.NotEmpty(t, key.Hash)
	assert.Equal(t, []string{"read", "write"}, key.Scopes)
}

func TestValidateAPIKey(t *testing.T) {
	s := setupTestStore(t)

	key, raw, err := s.Create("validate-test", []string{"read"})
	require.NoError(t, err)

	validated, err := s.Validate(raw)
	require.NoError(t, err)
	assert.Equal(t, key.ID, validated.ID)
	assert.Equal(t, key.Name, validated.Name)
	assert.Equal(t, key.Prefix, validated.Prefix)
	assert.Equal(t, []string{"read"}, validated.Scopes)

	// Invalid prefix
	_, err = s.Validate("invalid_key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid key format")

	// Non-existent key
	_, err = s.Validate("ak_00000000000000000000000000000000")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key not found")

	// Wrong key with valid prefix
	_, raw2, err := s.Create("another", []string{"read"})
	require.NoError(t, err)
	// tamper with the suffix
	wrongKey := raw2[:len(raw2)-1] + "x"
	_, err = s.Validate(wrongKey)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid key")
}

func TestHashAPIKey(t *testing.T) {
	raw := "ak_testhash1234567890123456789012"
	h := hashKey(raw)
	assert.NotEmpty(t, h)
	assert.NotEqual(t, raw, h)

	// Same raw key should still verify (bcrypt hashes differ but compare works)
	assert.True(t, checkKey(raw, h))

	// Different raw key should fail
	assert.False(t, checkKey("ak_differentkey9876543210987654321", h))
}

func TestRevokeAPIKey(t *testing.T) {
	s := setupTestStore(t)

	key, raw, err := s.Create("revoke-test", []string{"admin"})
	require.NoError(t, err)

	// Validate before revoke
	_, err = s.Validate(raw)
	require.NoError(t, err)

	// Revoke
	err = s.Revoke(key.ID)
	require.NoError(t, err)

	// Validate after revoke
	_, err = s.Validate(raw)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key revoked")
}
