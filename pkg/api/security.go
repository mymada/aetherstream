package api

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/labstack/echo/v4"
)

// InputValidator provides reusable validation helpers
type InputValidator struct{}

// ValidateUUID checks if string is a valid UUID v4 format
func (v *InputValidator) ValidateUUID(id string) error {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !re.MatchString(id) {
		return fmt.Errorf("invalid UUID format")
	}
	return nil
}

// ValidatePath ensures path is safe (no traversal)
func (v *InputValidator) ValidatePath(path string) error {
	clean := filepath.Clean(path)
	if strings.Contains(clean, "..") {
		return fmt.Errorf("path traversal detected")
	}
	if strings.HasPrefix(clean, "/") && !strings.HasPrefix(clean, "/media/") {
		return fmt.Errorf("absolute path not allowed")
	}
	return nil
}

// ValidateUsername ensures username is safe
func (v *InputValidator) ValidateUsername(username string) error {
	if len(username) < 3 || len(username) > 32 {
		return fmt.Errorf("username must be 3-32 characters")
	}
	re := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !re.MatchString(username) {
		return fmt.Errorf("username contains invalid characters")
	}
	return nil
}

// ValidateRole ensures role is valid
func (v *InputValidator) ValidateRole(role string) error {
	if role != "admin" && role != "user" {
		return fmt.Errorf("role must be 'admin' or 'user'")
	}
	return nil
}

// ValidateMediaType ensures media type is valid
func (v *InputValidator) ValidateMediaType(mt string) error {
	valid := map[string]bool{"movie": true, "tv": true, "music": true, "mixed": true, "photo": true}
	if !valid[mt] {
		return fmt.Errorf("invalid media type")
	}
	return nil
}

// SanitizeString removes dangerous characters
func SanitizeString(s string) string {
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// SecurityHeaders middleware adds OWASP recommended headers
func SecurityHeaders() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("X-Content-Type-Options", "nosniff")
			c.Response().Header().Set("X-Frame-Options", "DENY")
			c.Response().Header().Set("X-XSS-Protection", "1; mode=block")
			c.Response().Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			c.Response().Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'")
			return next(c)
		}
	}
}

// RateLimitPerRoute returns per-route rate limiter
func RateLimitPerRoute() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Simple IP-based rate limiting could be added here
			// For now, rely on Echo's global rate limiter
			return next(c)
		}
	}
}
