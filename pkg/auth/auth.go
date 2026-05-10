package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// Claims extends jwt.RegisteredClaims with AetherStream-specific fields
type Claims struct {
	jwt.RegisteredClaims
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"` // admin, user
	// SwiftFlow integration: device info from captive portal
	DeviceID    string `json:"device_id,omitempty"`
	BandwidthKB int    `json:"bandwidth_kb,omitempty"` // from SwiftFlow QoS
}

// Service handles JWT generation and validation
type Service struct {
	secret []byte
	ttl    time.Duration
}

// NewService creates auth service with secret from config
func NewService(secret string, ttlHours int) (*Service, error) {
	if len(secret) < 32 {
		return nil, errors.New("auth secret must be at least 32 characters")
	}
	return &Service{
		secret: []byte(secret),
		ttl:    time.Duration(ttlHours) * time.Hour,
	}, nil
}

// GenerateToken creates JWT for user
func (s *Service) GenerateToken(userID, username, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
			Issuer:    "aetherstream",
		},
		UserID:   userID,
		Username: username,
		Role:     role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// ValidateToken parses and verifies JWT
func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token claims")
}

// Middleware returns Echo JWT middleware
func (s *Service) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return echo.NewHTTPError(401, "missing authorization header")
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				return echo.NewHTTPError(401, "invalid authorization header format")
			}

			claims, err := s.ValidateToken(parts[1])
			if err != nil {
				return echo.NewHTTPError(401, "invalid token: "+err.Error())
			}

			c.Set("user", claims)
			return next(c)
		}
	}
}

// RequireRole middleware checks user role
func RequireRole(role string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, ok := c.Get("user").(*Claims)
			if !ok {
				return echo.NewHTTPError(403, "unauthorized")
			}
			if user.Role != "admin" && user.Role != role {
				return echo.NewHTTPError(403, "insufficient privileges")
			}
			return next(c)
		}
	}
}

// GetUser extracts Claims from Echo context
func GetUser(c echo.Context) *Claims {
	if user, ok := c.Get("user").(*Claims); ok {
		return user
	}
	return nil
}
