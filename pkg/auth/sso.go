package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/oauth2"
)

// SSOProvider defines the interface for SSO authentication backends.
type SSOProvider interface {
	Name() string
	AuthURL(state string) string
	Exchange(code string) (*UserInfo, error)
}

// SAMLProvider is a placeholder for SAML 2.0 identity provider integration.
// In production this would use a library like crewjam/saml or russellhaering/gosaml2.
type SAMLProvider struct {
	metadataURL string
	entityID    string
	acsURL      string
}

// NewSAMLProvider creates a SAML provider configuration.
func NewSAMLProvider(metadataURL, entityID, acsURL string) *SAMLProvider {
	return &SAMLProvider{
		metadataURL: metadataURL,
		entityID:    entityID,
		acsURL:      acsURL,
	}
}

// Name returns the provider name.
func (s *SAMLProvider) Name() string { return "saml" }

// AuthURL returns a placeholder SAML SSO URL.
func (s *SAMLProvider) AuthURL(state string) string {
	u, _ := url.Parse(s.metadataURL)
	q := u.Query()
	q.Set("SAMLRequest", base64.URLEncoding.EncodeToString([]byte(state)))
	q.Set("RelayState", state)
	u.RawQuery = q.Encode()
	return u.String()
}

// Exchange is a placeholder for SAML assertion exchange.
func (s *SAMLProvider) Exchange(assertion string) (*UserInfo, error) {
	// In a real implementation, parse and validate the SAML assertion,
	// extract NameID and attributes, then return UserInfo.
	return &UserInfo{
		Provider: "saml",
		Email:    "user@example.com",
		Name:     "SAML User",
	}, nil
}

// LDAPProvider is a placeholder for LDAP/Active Directory authentication.
type LDAPProvider struct {
	addr       string
	baseDN     string
	bindDN     string
	bindPass   string
	userFilter string
}

// NewLDAPProvider creates an LDAP provider configuration.
func NewLDAPProvider(addr, baseDN, bindDN, bindPass, userFilter string) *LDAPProvider {
	if userFilter == "" {
		userFilter = "(uid=%s)"
	}
	return &LDAPProvider{
		addr:       addr,
		baseDN:     baseDN,
		bindDN:     bindDN,
		bindPass:   bindPass,
		userFilter: userFilter,
	}
}

// Name returns the provider name.
func (l *LDAPProvider) Name() string { return "ldap" }

// AuthURL returns empty because LDAP is not redirect-based.
func (l *LDAPProvider) AuthURL(state string) string { return "" }

// Exchange validates LDAP credentials (username/password) and returns UserInfo.
// In production this would use github.com/go-ldap/ldap/v3.
func (l *LDAPProvider) Exchange(credentials string) (*UserInfo, error) {
	parts := strings.SplitN(credentials, ":", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid credentials format")
	}
	username, password := parts[0], parts[1]
	_ = password
	// Placeholder: real implementation would dial LDAP, bind, search, verify password.
	return &UserInfo{
		Provider: l.Name(),
		Email:    username + "@" + l.addr,
		Name:     username,
	}, nil
}

// OAuthSSOProvider wraps golang.org/x/oauth2 for generic OAuth2/OIDC SSO.
type OAuthSSOProvider struct {
	name         string
	config       *oauth2.Config
	userInfoURL  string
}

// NewOAuthSSOProvider creates an OAuth2/OIDC SSO provider.
func NewOAuthSSOProvider(name, clientID, clientSecret, authURL, tokenURL, redirectURL, userInfoURL string) *OAuthSSOProvider {
	return &OAuthSSOProvider{
		name: name,
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret:   clientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  authURL,
				TokenURL: tokenURL,
			},
			RedirectURL: redirectURL,
			Scopes:      []string{"openid", "profile", "email"},
		},
		userInfoURL: userInfoURL,
	}
}

// Name returns the provider name.
func (o *OAuthSSOProvider) Name() string { return o.name }

// AuthURL returns the OAuth2 authorization URL.
func (o *OAuthSSOProvider) AuthURL(state string) string {
	return o.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// Exchange exchanges an authorization code for an access token and fetches user info.
func (o *OAuthSSOProvider) Exchange(code string) (*UserInfo, error) {
	ctx := oauth2.NoContext
	token, err := o.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oauth exchange: %w", err)
	}

	req, err := http.NewRequest("GET", o.userInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo endpoint returned %d", resp.StatusCode)
	}

	var info struct {
		Email string `json:"email"`
		Name  string `json:"name"`
		Sub   string `json:"sub"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	return &UserInfo{
		Provider: o.name,
		Email:    info.Email,
		Name:     info.Name,
		ExternalID: info.Sub,
	}, nil
}

// UserInfo holds normalized user information from any SSO provider.
type UserInfo struct {
	Provider   string `json:"provider"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	ExternalID string `json:"external_id"`
}

// SSOService manages multiple SSO providers and JWT generation.
type SSOService struct {
	providers map[string]SSOProvider
	jwtSecret []byte
	ttl       time.Duration
}

// NewSSOService creates an SSO service with a JWT secret.
func NewSSOService(secret string, ttlHours int) (*SSOService, error) {
	if len(secret) < 32 {
		return nil, errors.New("sso secret must be at least 32 characters")
	}
	return &SSOService{
		providers: make(map[string]SSOProvider),
		jwtSecret: []byte(secret),
		ttl:       time.Duration(ttlHours) * time.Hour,
	}, nil
}

// RegisterProvider adds an SSO provider.
func (s *SSOService) RegisterProvider(p SSOProvider) {
	s.providers[p.Name()] = p
}

// GetProvider returns a registered provider by name.
func (s *SSOService) GetProvider(name string) (SSOProvider, bool) {
	p, ok := s.providers[name]
	return p, ok
}

// GenerateToken creates a JWT for an SSO-authenticated user.
func (s *SSOService) GenerateToken(userID, username, role string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":      userID,
		"iat":      jwt.NewNumericDate(now),
		"exp":      jwt.NewNumericDate(now.Add(s.ttl)),
		"iss":      "aetherstream-sso",
		"user_id":  userID,
		"username": username,
		"role":     role,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// ValidateToken parses and verifies a JWT.
func (s *SSOService) ValidateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token claims")
}

// GenerateState creates a random state parameter for OAuth/SAML flows.
func GenerateState() (string, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// SSOHandler returns Echo handlers for SSO login and callback.
func (s *SSOService) SSOHandler(providerName string) echo.HandlerFunc {
	return func(c echo.Context) error {
		p, ok := s.GetProvider(providerName)
		if !ok {
			return echo.NewHTTPError(http.StatusBadRequest, "unknown provider")
		}
		state, err := GenerateState()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "state generation failed")
		}
		// Store state in cookie for callback validation
		c.SetCookie(&http.Cookie{
			Name:     "sso_state",
			Value:    state,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   600,
			Path:     "/",
		})
		return c.Redirect(http.StatusFound, p.AuthURL(state))
	}
}

// SSOCallbackHandler handles the OAuth/SAML callback.
func (s *SSOService) SSOCallbackHandler(providerName string) echo.HandlerFunc {
	return func(c echo.Context) error {
		p, ok := s.GetProvider(providerName)
		if !ok {
			return echo.NewHTTPError(http.StatusBadRequest, "unknown provider")
		}
		code := c.QueryParam("code")
		if code == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "missing code")
		}
		userInfo, err := p.Exchange(code)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "sso exchange failed: "+err.Error())
		}
		// In production, lookup or create the user in the database here.
		token, err := s.GenerateToken(userInfo.ExternalID, userInfo.Email, "user")
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "token generation failed")
		}
		return c.JSON(http.StatusOK, map[string]string{
			"token":    token,
			"provider": providerName,
			"email":    userInfo.Email,
		})
	}
}
