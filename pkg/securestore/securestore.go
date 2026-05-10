package securestore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
)

// Store handles AES-256-GCM encryption of secrets
type Store struct {
	key []byte
}

// NewStore creates a secure store from a 32-byte key (env or derived)
func NewStore(key string) (*Store, error) {
	if len(key) < 32 {
		return nil, errors.New("securestore key must be at least 32 characters")
	}
	// Use first 32 bytes as AES-256 key
	k := []byte(key)[:32]
	return &Store{key: k}, nil
}

// NewStoreFromEnv creates store from environment variable
func NewStoreFromEnv(envVar string) (*Store, error) {
	key := os.Getenv(envVar)
	if key == "" {
		return nil, fmt.Errorf("environment variable %s not set", envVar)
	}
	return NewStore(key)
}

// Encrypt encrypts plaintext with AES-256-GCM, returns base64 ciphertext
func (s *Store) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64 ciphertext with AES-256-GCM
func (s *Store) Decrypt(ciphertextB64 string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}

// SecureCompare constant-time comparison to prevent timing attacks
func SecureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
