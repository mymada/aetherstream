package auth

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/pquerna/otp/totp"
)

// TOTPService handles 2FA TOTP generation and validation
type TOTPService struct{}

// NewTOTPService creates a TOTP service
func NewTOTPService() *TOTPService {
	return &TOTPService{}
}

// GenerateSecret creates a new TOTP secret for a user
func (s *TOTPService) GenerateSecret(userID string) (string, string, error) {
	secret := make([]byte, 20)
	if _, err := rand.Read(secret); err != nil {
		return "", "", fmt.Errorf("generate secret: %w", err)
	}
	secretBase32 := base32.StdEncoding.EncodeToString(secret)

	// Generate provisioning URI for QR code
	uri := totp.GenerateOpts{
		Issuer:      "AetherStream",
		AccountName: userID,
		Secret:      secret,
	}
	key, err := totp.Generate(uri)
	if err != nil {
		return "", "", fmt.Errorf("generate key: %w", err)
	}

	return secretBase32, key.URL(), nil
}

// ValidateCode verifies a TOTP code against a secret
func (s *TOTPService) ValidateCode(secret, code string) bool {
	return totp.Validate(code, secret)
}

// GenerateCode generates a TOTP code (for testing)
func (s *TOTPService) GenerateCode(secret string) (string, error) {
	return totp.GenerateCode(secret, time.Now())
}

// GenerateBackupCodes creates 10 single-use backup codes
func (s *TOTPService) GenerateBackupCodes() ([]string, error) {
	codes := make([]string, 10)
	for i := range codes {
		b := make([]byte, 4)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		// Format: XXXX-XXXX (8 chars)
		codes[i] = fmt.Sprintf("%04d-%04d", binary.BigEndian.Uint16(b[:2])%10000, binary.BigEndian.Uint16(b[2:])%10000)
	}
	return codes, nil
}

// HashBackupCode hashes a backup code for storage
func HashBackupCode(code string) string {
	// Simple hash - in production use bcrypt
	return fmt.Sprintf("%x", code)
}
