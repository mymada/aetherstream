package auth

import (
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService_ShortSecret(t *testing.T) {
	_, err := NewService("short", 24)
	assert.Error(t, err)
}

func TestGenerateAndValidateToken(t *testing.T) {
	svc, err := NewService("this-is-a-very-long-secret-key-32chars", 24)
	require.NoError(t, err)

	token, err := svc.GenerateToken("u1", "alice", "admin")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := svc.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, "u1", claims.UserID)
	assert.Equal(t, "alice", claims.Username)
	assert.Equal(t, "admin", claims.Role)
	assert.Equal(t, "aetherstream", claims.Issuer)
}

func TestValidateToken_Expired(t *testing.T) {
	svc, err := NewService("this-is-a-very-long-secret-key-32chars", -1) // expired immediately
	require.NoError(t, err)

	token, err := svc.GenerateToken("u1", "alice", "admin")
	require.NoError(t, err)

	_, err = svc.ValidateToken(token)
	assert.Error(t, err)
}

func TestValidateToken_Invalid(t *testing.T) {
	svc, err := NewService("this-is-a-very-long-secret-key-32chars", 24)
	require.NoError(t, err)

	_, err = svc.ValidateToken("invalid.token.here")
	assert.Error(t, err)
}

func TestRequireRole(t *testing.T) {
	// Create a minimal echo context for testing
	e := echo.New()
	req := e.NewContext(nil, nil)
	
	// Test with no user in context
	middleware := RequireRole("admin")
	err := middleware(func(c echo.Context) error { return nil })(req)
	assert.Error(t, err)
	assert.Equal(t, 403, err.(*echo.HTTPError).Code)
}
