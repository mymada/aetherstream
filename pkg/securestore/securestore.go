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
	return &Store{key: deriveKey(password, nil)}, nil
}

// deriveKey generates a 32-byte key via PBKDF2.
// If salt is nil, a random 16-byte salt is generated and prepended to the key.
func deriveKey(password string, salt []byte) []byte {
	if salt == nil {
		salt = make([]byte, 16)
		if _, err := io.ReadFull(rand.Reader, salt); err != nil {
			panic(fmt.Sprintf("failed to generate salt: %v", err))
		}
	}
	key := pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)
	return append(salt, key...)
}

// NewStoreFromEnv creates store from environment variable, deriving key via PBKDF2.
func NewStoreFromEnv(envVar string) (*Store, error) {
	password := os.Getenv(envVar)
	if password == "" {
		return nil, fmt.Errorf("environment variable %s not set", envVar)
	}
	return NewStore(password)
}

// Encrypt encrypts plaintext with AES-256-GCM, returns base64 ciphertext.
// The salt (first 16 bytes of s.key) is embedded in the output via the key derivation.
func (s *Store) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(s.key[16:])
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

	block, err := aes.NewCipher(s.key[16:])
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
