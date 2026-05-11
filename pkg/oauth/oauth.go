package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

// ProviderConfig holds OAuth2 settings for a provider
type ProviderConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURL  string `json:"redirect_url"`
	Enabled      bool   `json:"enabled"`
}

// Config holds all OAuth2 provider configs
type Config struct {
	Google ProviderConfig `json:"google"`
	GitHub ProviderConfig `json:"github"`
}

// UserInfo represents normalized user data from OAuth providers
type UserInfo struct {
	Provider string `json:"provider"`
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Picture  string `json:"picture,omitempty"`
}

// Service handles OAuth2 flows
type Service struct {
	cfg       Config
	logger    zerolog.Logger
	states    map[string]time.Time // state -> expiry
	stateMu   sync.RWMutex
	providers map[string]*oauth2.Config
	stopClean chan struct{}        // signals cleanup goroutine
}

// NewService creates OAuth service with provider configs
func NewService(cfg Config) *Service {
	s := &Service{
		cfg:       cfg,
		logger:    zerolog.New(nil),
		states:    make(map[string]time.Time),
		providers: make(map[string]*oauth2.Config),
		stopClean: make(chan struct{}),
	}

	// Start periodic cleanup of expired OAuth states
	go s.cleanupLoop()

	if cfg.Google.Enabled {
		s.providers["google"] = &oauth2.Config{
			ClientID:     cfg.Google.ClientID,
			ClientSecret: cfg.Google.ClientSecret,
			RedirectURL:  cfg.Google.RedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		}
	}

	if cfg.GitHub.Enabled {
		s.providers["github"] = &oauth2.Config{
			ClientID:     cfg.GitHub.ClientID,
			ClientSecret: cfg.GitHub.ClientSecret,
			RedirectURL:  cfg.GitHub.RedirectURL,
			Scopes:       []string{"read:user", "user:email"},
			Endpoint:     github.Endpoint,
		}
	}

	return s
}

// EnabledProviders returns list of configured providers
func (s *Service) EnabledProviders() []string {
	var names []string
	for name := range s.providers {
		names = append(names, name)
	}
	return names
}

// IsEnabled checks if a provider is configured
func (s *Service) IsEnabled(provider string) bool {
	_, ok := s.providers[provider]
	return ok
}

// GenerateState creates a random state parameter for CSRF protection
func (s *Service) GenerateState() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	state := base64.URLEncoding.EncodeToString(b)
	s.stateMu.Lock()
	s.states[state] = time.Now().Add(10 * time.Minute)
	s.stateMu.Unlock()
	return state
}

// ValidateState checks and consumes a state parameter
func (s *Service) ValidateState(state string) bool {
	s.stateMu.Lock()
	expiry, ok := s.states[state]
	if ok {
		delete(s.states, state)
	}
	s.stateMu.Unlock()
	return ok && time.Now().Before(expiry)
}

// AuthURL returns the authorization URL for a provider
func (s *Service) AuthURL(provider string) (string, error) {
	p, ok := s.providers[provider]
	if !ok {
		return "", fmt.Errorf("provider %s not enabled", provider)
	}
	state := s.GenerateState()
	return p.AuthCodeURL(state, oauth2.AccessTypeOnline), nil
}

// Exchange exchanges code for token and fetches user info
func (s *Service) Exchange(ctx context.Context, provider, code string) (*UserInfo, error) {
	p, ok := s.providers[provider]
	if !ok {
		return nil, fmt.Errorf("provider %s not enabled", provider)
	}

	token, err := p.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	client := p.Client(ctx, token)

	switch provider {
	case "google":
		return s.fetchGoogleUser(client)
	case "github":
		return s.fetchGitHubUser(ctx, client)
	default:
		return nil, fmt.Errorf("unsupported provider %s", provider)
	}
}

func (s *Service) fetchGoogleUser(client *http.Client) (*UserInfo, error) {
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google userinfo: %s", resp.Status)
	}

	var data struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	return &UserInfo{
		Provider: "google",
		ID:       data.ID,
		Email:    data.Email,
		Name:     data.Name,
		Picture:  data.Picture,
	}, nil
}

func (s *Service) fetchGitHubUser(ctx context.Context, client *http.Client) (*UserInfo, error) {
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github user: %s", resp.Status)
	}

	var data struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
		Email string `json:"email"`
		Avatar string `json:"avatar_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	// GitHub may not expose email in /user if private; try /user/emails
	email := data.Email
	if email == "" {
		email = s.fetchGitHubPrimaryEmail(ctx, client)
	}

	name := data.Name
	if name == "" {
		name = data.Login
	}

	return &UserInfo{
		Provider: "github",
		ID:       fmt.Sprintf("%d", data.ID),
		Email:    email,
		Name:     name,
		Picture:  data.Avatar,
	}, nil
}

func (s *Service) fetchGitHubPrimaryEmail(ctx context.Context, client *http.Client) string {
	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return ""
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email
		}
	}
	if len(emails) > 0 {
		return emails[0].Email
	}
	return ""
}

// RegisterRoutes sets up OAuth2 endpoints on Echo
func (s *Service) RegisterRoutes(e *echo.Echo, callbackHandler echo.HandlerFunc) {
	// GET /auth/oauth/:provider/login — redirect to provider
	e.GET("/auth/oauth/:provider/login", func(c echo.Context) error {
		provider := c.Param("provider")
		if !s.IsEnabled(provider) {
			return echo.NewHTTPError(http.StatusBadRequest, "provider not enabled")
		}
		url, err := s.AuthURL(provider)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.Redirect(http.StatusTemporaryRedirect, url)
	})

	// GET /auth/oauth/:provider/callback — provider redirects here
	e.GET("/auth/oauth/:provider/callback", callbackHandler)
}

// cleanupLoop periodically removes expired OAuth states every 5 minutes
func (s *Service) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.purgeExpiredStates()
		case <-s.stopClean:
			return
		}
	}
}

// purgeExpiredStates deletes all expired states from the map
func (s *Service) purgeExpiredStates() {
	now := time.Now()
	s.stateMu.Lock()
	for state, expiry := range s.states {
		if now.After(expiry) {
			delete(s.states, state)
		}
	}
	s.stateMu.Unlock()
}

// Stop halts the background cleanup goroutine
func (s *Service) Stop() {
	close(s.stopClean)
}
