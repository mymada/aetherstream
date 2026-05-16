package oauth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateState_Invalid(t *testing.T) {
	svc := NewService(Config{})
	defer svc.Stop()
	assert.False(t, svc.ValidateState("nonexistent-state"))
}

func TestValidateState_Consumed(t *testing.T) {
	svc := NewService(Config{})
	defer svc.Stop()
	state := svc.GenerateState()
	assert.True(t, svc.ValidateState(state))
	// Second call with same state must fail — it was consumed
	assert.False(t, svc.ValidateState(state))
}

func TestStop_Idempotent(t *testing.T) {
	svc := NewService(Config{})
	assert.NotPanics(t, func() {
		svc.Stop()
		svc.Stop()
		svc.Stop()
	})
}

func TestBothProviders_Enabled(t *testing.T) {
	svc := NewService(Config{
		Google: ProviderConfig{ClientID: "gid", ClientSecret: "gs", RedirectURL: "http://localhost/g", Enabled: true},
		GitHub: ProviderConfig{ClientID: "hid", ClientSecret: "hs", RedirectURL: "http://localhost/h", Enabled: true},
	})
	defer svc.Stop()
	providers := svc.EnabledProviders()
	assert.Len(t, providers, 2)
	assert.True(t, svc.IsEnabled("google"))
	assert.True(t, svc.IsEnabled("github"))
}

func TestAuthURL_GitHub(t *testing.T) {
	svc := NewService(Config{
		GitHub: ProviderConfig{ClientID: "cid", ClientSecret: "cs", RedirectURL: "http://localhost/cb", Enabled: true},
	})
	defer svc.Stop()
	url, err := svc.AuthURL("github")
	require.NoError(t, err)
	assert.Contains(t, url, "github.com")
	assert.Contains(t, url, "state=")
}

func TestIsEnabled_Unknown(t *testing.T) {
	svc := NewService(Config{})
	defer svc.Stop()
	assert.False(t, svc.IsEnabled("twitter"))
	assert.False(t, svc.IsEnabled(""))
}

func TestGenerateState_IsUnique(t *testing.T) {
	svc := NewService(Config{})
	defer svc.Stop()
	s1 := svc.GenerateState()
	s2 := svc.GenerateState()
	assert.NotEmpty(t, s1)
	assert.NotEmpty(t, s2)
	assert.NotEqual(t, s1, s2)
}

func TestPurgeExpiredStates(t *testing.T) {
	svc := NewService(Config{})
	defer svc.Stop()

	svc.stateMu.Lock()
	svc.states["expired"] = time.Now().Add(-1 * time.Minute)
	svc.states["valid"] = time.Now().Add(10 * time.Minute)
	svc.stateMu.Unlock()

	svc.purgeExpiredStates()

	svc.stateMu.RLock()
	_, hasExpired := svc.states["expired"]
	_, hasValid := svc.states["valid"]
	svc.stateMu.RUnlock()

	assert.False(t, hasExpired, "expired state should be removed")
	assert.True(t, hasValid, "valid state should remain")
}

func TestAuthURL_UnknownProvider_Error(t *testing.T) {
	svc := NewService(Config{
		Google: ProviderConfig{Enabled: true, ClientID: "g", ClientSecret: "s", RedirectURL: "http://x"},
	})
	defer svc.Stop()
	_, err := svc.AuthURL("twitter")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "twitter")
}
