package securestore

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncryptDecrypt(t *testing.T) {
	store, err := NewStore("this-is-a-32-byte-key-for-testing!!")
	assert.NoError(t, err)

	plaintext := "my-secret-jwt-key-1234567890"
	encrypted, err := store.Encrypt(plaintext)
	assert.NoError(t, err)
	assert.NotEmpty(t, encrypted)
	assert.NotEqual(t, plaintext, encrypted)

	decrypted, err := store.Decrypt(encrypted)
	assert.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestDecryptTampered(t *testing.T) {
	store, err := NewStore("this-is-a-32-byte-key-for-testing!!")
	assert.NoError(t, err)

	encrypted, _ := store.Encrypt("secret")
	// Tamper with ciphertext
	tampered := encrypted[:len(encrypted)-1] + "X"

	_, err = store.Decrypt(tampered)
	assert.Error(t, err)
}

func TestSecureCompare(t *testing.T) {
	assert.True(t, SecureCompare("same", "same"))
	assert.False(t, SecureCompare("a", "b"))
}

func TestNewStoreShortKey(t *testing.T) {
	_, err := NewStore("short")
	assert.Error(t, err)
}
