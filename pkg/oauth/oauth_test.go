package oauth

import (
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService_NoProviders(t *testing.T) {
	svc := NewService(Config{})
	assert.Empty(t, svc.EnabledProviders())
	assert.False(t, svc.IsEnabled("google"))
	assert.False(t, svc.IsEnabled("github"))
}

func TestNewService_GoogleEnabled(t *testing.T) {
	svc := NewService(Config{
		Google: ProviderConfig{ClientID: "cid", ClientSecret: "cs", RedirectURL: "http://localhost/cb", Enabled: true},
	})
	assert.Contains(t, svc.EnabledProviders(), "google")
	assert.True(t, svc.IsEnabled("google"))
	assert.False(t, svc.IsEnabled("github"))
}

func TestNewService_GitHubEnabled(t *testing.T) {
	svc := NewService(Config{
		GitHub: ProviderConfig{ClientID: "cid", ClientSecret: "cs", RedirectURL: "http://localhost/cb", Enabled: true},
	})
	assert.Contains(t, svc.EnabledProviders(), "github")
	assert.True(t, svc.IsEnabled("github"))
	assert.False(t, svc.IsEnabled("google"))
}

func TestGenerateAndValidateState(t *testing.T) {
	svc := NewService(Config{})
	state := svc.GenerateState()
	require.NotEmpty(t, state)
	assert.True(t, svc.ValidateState(state))
	// second use should fail
	assert.False(t, svc.ValidateState(state))
}

func TestAuthURL(t *testing.T) {
	svc := NewService(Config{
		Google: ProviderConfig{ClientID: "cid", ClientSecret: "cs", RedirectURL: "http://localhost/cb", Enabled: true},
	})
	url, err := svc.AuthURL("google")
	require.NoError(t, err)
	assert.Contains(t, url, "accounts.google.com")
	assert.Contains(t, url, "state=")
}

func TestAuthURL_UnknownProvider(t *testing.T) {
	svc := NewService(Config{})
	_, err := svc.AuthURL("google")
	assert.Error(t, err)
}

func TestRegisterRoutes(t *testing.T) {
	svc := NewService(Config{
		Google: ProviderConfig{ClientID: "cid", ClientSecret: "cs", RedirectURL: "http://localhost/cb", Enabled: true},
	})
	e := echo.New()
	svc.RegisterRoutes(e, func(c echo.Context) error { return nil })

	routes := e.Routes()
	var hasLogin, hasCallback bool
	for _, r := range routes {
		if r.Path == "/auth/oauth/:provider/login" {
			hasLogin = true
		}
		if r.Path == "/auth/oauth/:provider/callback" {
			hasCallback = true
		}
	}
	assert.True(t, hasLogin, "login route missing")
	assert.True(t, hasCallback, "callback route missing")
}
