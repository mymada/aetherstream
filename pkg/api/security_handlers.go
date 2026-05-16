package api

import (
	"net/http"
	"time"

	"github.com/devuser/aetherstream/pkg/apikeys"
	"github.com/devuser/aetherstream/pkg/audit"
	authpkg "github.com/devuser/aetherstream/pkg/auth"
	"github.com/labstack/echo/v4"
)

// AuditLogEntry represents a security audit log entry.
type AuditLogEntry struct {
	UserID     string    `json:"userId"`
	Username   string    `json:"username"`
	Action     string    `json:"action"`
	Resource   string    `json:"resource"`
	ResourceID string    `json:"resourceId"`
	IP         string    `json:"ip"`
	UserAgent  string    `json:"userAgent"`
	Details    string    `json:"details"`
	CreatedAt  time.Time `json:"createdAt"`
}

// handleAuditLogs returns recent audit log entries (admin only).
func (s *Server) handleAuditLogs(c echo.Context) error {
	logger := audit.NewLogger(s.db.DB)
	if err := logger.Migrate(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "audit table: "+err.Error())
	}
	events, err := logger.Query(200)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	out := make([]AuditLogEntry, 0, len(events))
	for _, e := range events {
		out = append(out, AuditLogEntry{
			UserID:     e.UserID,
			Username:   e.Username,
			Action:     e.Action,
			Resource:   e.Resource,
			ResourceID: e.ResourceID,
			IP:         e.IP,
			UserAgent:  e.UserAgent,
			Details:    e.Details,
			CreatedAt:  e.Timestamp,
		})
	}
	return c.JSON(http.StatusOK, out)
}

// handleEnable2FA generates a TOTP secret and QR URL for the authenticated user.
func (s *Server) handleEnable2FA(c echo.Context) error {
	claims := authpkg.GetUser(c)
	if claims == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS user_totp (
			user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			encrypted_secret TEXT NOT NULL,
			verified INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "totp table: "+err.Error())
	}

	totpSvc := authpkg.NewTOTPService()
	secretBase32, qrURL, err := totpSvc.GenerateSecret(claims.Username)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "generate secret: "+err.Error())
	}

	encrypted, err := s.secureStore.Encrypt(secretBase32)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "encrypt secret: "+err.Error())
	}

	if _, err := s.db.Exec(
		`INSERT INTO user_totp(user_id, encrypted_secret, verified) VALUES(?, ?, 0)
		 ON CONFLICT(user_id) DO UPDATE SET encrypted_secret=excluded.encrypted_secret, verified=0`,
		claims.UserID, encrypted,
	); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "store secret: "+err.Error())
	}

	backupCodes, err := totpSvc.GenerateBackupCodes()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "backup codes: "+err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"secret":      secretBase32,
		"qrUrl":       qrURL,
		"backupCodes": backupCodes,
		"message":     "Scan the QR code then POST {\"code\":\"123456\"} to /api/auth/2fa/verify to activate",
	})
}

// handleVerify2FA confirms a TOTP code and marks 2FA as active for the user.
func (s *Server) handleVerify2FA(c echo.Context) error {
	claims := authpkg.GetUser(c)
	if claims == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := c.Bind(&body); err != nil || body.Code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "code required")
	}

	var encryptedSecret string
	if err := s.db.QueryRow(
		"SELECT encrypted_secret FROM user_totp WHERE user_id = ?", claims.UserID,
	).Scan(&encryptedSecret); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "2FA not initialized — call /api/auth/2fa/enable first")
	}

	secretBase32, err := s.secureStore.Decrypt(encryptedSecret)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "decrypt secret: "+err.Error())
	}

	totpSvc := authpkg.NewTOTPService()
	if !totpSvc.ValidateCode(secretBase32, body.Code) {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired TOTP code")
	}

	if _, err := s.db.Exec(
		"UPDATE user_totp SET verified = 1 WHERE user_id = ?", claims.UserID,
	); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "activate 2FA: "+err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"enabled": true,
		"message": "2FA successfully enabled",
	})
}

// handleAPIKeys handles listing, creation, and revocation of API keys.
// GET  /api/api-keys         — list all keys (no hashes)
// POST /api/api-keys         — create key, returns raw key once
// DELETE /api/api-keys?id=X  — revoke key
func (s *Server) handleAPIKeys(c echo.Context) error {
	store := apikeys.NewStore(s.db.DB)
	if err := store.Migrate(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "api keys table: "+err.Error())
	}

	switch c.Request().Method {
	case http.MethodGet:
		keys, err := store.List()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		if keys == nil {
			keys = []apikeys.Key{}
		}
		return c.JSON(http.StatusOK, keys)

	case http.MethodPost:
		var body struct {
			Name   string   `json:"name"`
			Scopes []string `json:"scopes"`
		}
		if err := c.Bind(&body); err != nil || body.Name == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "name required")
		}
		if len(body.Scopes) == 0 {
			body.Scopes = []string{"read"}
		}
		key, rawKey, err := store.Create(body.Name, body.Scopes)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusCreated, map[string]interface{}{
			"key":    rawKey,
			"id":     key.ID,
			"name":   key.Name,
			"scopes": key.Scopes,
			"prefix": key.Prefix,
			"note":   "Save this key — it will not be shown again",
		})

	case http.MethodDelete:
		id := c.QueryParam("id")
		if id == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "id required")
		}
		if err := store.Revoke(id); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.NoContent(http.StatusNoContent)

	default:
		return echo.NewHTTPError(http.StatusMethodNotAllowed, "method not allowed")
	}
}
