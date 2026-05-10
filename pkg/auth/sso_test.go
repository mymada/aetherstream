package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSAMLProvider(t *testing.T) {
	p := NewSAMLProvider("https://idp.example.com/metadata", "aetherstream", "https://aetherstream.example.com/saml/acs")
	assert.Equal(t, "saml", p.Name())
	assert.NotEmpty(t, p.AuthURL("state123"))

	info, err := p.Exchange("assertion")
	assert.NoError(t, err)
	assert.Equal(t, "saml", info.Provider)
}

func TestNewLDAPProvider(t *testing.T) {
	p := NewLDAPProvider("ldap.example.com:636", "dc=example,dc=com", "cn=admin,dc=example,dc=com", "secret", "")
	assert.Equal(t, "ldap", p.Name())
	assert.Empty(t, p.AuthURL("state"))

	info, err := p.Exchange("user:pass")
	assert.NoError(t, err)
	assert.Equal(t, "ldap", info.Provider)
	assert.Equal(t, "user@ldap.example.com:636", info.Email)
}

func TestNewLDAPProvider_InvalidCredentials(t *testing.T) {
	p := NewLDAPProvider("ldap.example.com", "dc=example,dc=com", "", "", "")
	_, err := p.Exchange("badformat")
	assert.Error(t, err)
}

func TestGenerateState(t *testing.T) {
	state, err := GenerateState()
	assert.NoError(t, err)
	assert.NotEmpty(t, state)
}

func TestNewSSOService(t *testing.T) {
	_, err := NewSSOService("short", 24)
	assert.Error(t, err)

	svc, err := NewSSOService("this-is-a-very-long-secret-key-32bytes", 24)
	assert.NoError(t, err)
	assert.NotNil(t, svc)

	// Register and retrieve provider
	p := NewSAMLProvider("https://idp.example.com/metadata", "aetherstream", "https://aetherstream.example.com/saml/acs")
	svc.RegisterProvider(p)

	got, ok := svc.GetProvider("saml")
	assert.True(t, ok)
	assert.Equal(t, "saml", got.Name())

	_, ok = svc.GetProvider("nonexistent")
	assert.False(t, ok)
}

func TestSSOService_GenerateAndValidateToken(t *testing.T) {
	svc, _ := NewSSOService("this-is-a-very-long-secret-key-32bytes", 24)
	token, err := svc.GenerateToken("user-1", "alice", "user")
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := svc.ValidateToken(token)
	assert.NoError(t, err)
	assert.Equal(t, "user-1", claims["user_id"])
	assert.Equal(t, "alice", claims["username"])
	assert.Equal(t, "user", claims["role"])
}

func TestSSOService_ValidateToken_Invalid(t *testing.T) {
	svc, _ := NewSSOService("this-is-a-very-long-secret-key-32bytes", 24)
	_, err := svc.ValidateToken("bad.token.here")
	assert.Error(t, err)
}

func TestOAuthSSOProvider_Name(t *testing.T) {
	p := NewOAuthSSOProvider("google", "cid", "csec", "https://auth", "https://token", "https://redirect", "https://userinfo")
	assert.Equal(t, "google", p.Name())
	assert.NotEmpty(t, p.AuthURL("state123"))
}
