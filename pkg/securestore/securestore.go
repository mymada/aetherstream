package securestore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/pbkdf2"
)

// Store handles AES-256-GCM encryption of secrets
type Store struct {
	key []byte
}

// NewStore creates a secure store from a password string, deriving a 32-byte key via PBKDF2.
func NewStore(password string) (*Store, error) {
	if len(password) < 8 {
		return nil, errors.New("securestore password must be at least 8 characters")
	}
	salt := []byte("aetherstream-static-salt-v1")
	key := pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)
	return &Store{key: key}, nil
}

// NewStoreFromEnv creates store from environment variable, deriving key via PBKDF2.
func NewStoreFromEnv(envVar string) (*Store, error) {
	password := os.Getenv(envVar)
	if password == "" {
		return nil, fmt.Errorf("environment variable %s not set", envVar)
	}
	return NewStore(password)
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
