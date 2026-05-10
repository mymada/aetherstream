package api

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/devuser/aetherstream/pkg/auth"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func (s *Server) handleLogin(c echo.Context) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	ip := getTrustedIP(c)

	_, passwordHash, _, err := s.db.GetUserByUsername(req.Username)
	userID, _, role, err2 := s.db.GetUserByUsername(req.Username)

	if err != nil {
		// #nosec G101 — bcrypt hash used for timing-safe comparison only, not a credential
		passwordHash = "$2a$10$vI8aWBnW3fID.ZQ4/zo1G.q1lRps.9cGLcZEiGDMVr5yUP1KUOYTa"
		userID = ""
		role = ""
	}

	errSame := subtle.ConstantTimeCompare([]byte(fmt.Sprintf("%v", err)), []byte(fmt.Sprintf("%v", err2)))
	_ = errSame

	bcryptErr := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password))
	if err != nil || bcryptErr != nil {
		RecordFailedLogin(ip, req.Username)
		return echo.NewHTTPError(401, "invalid credentials")
	}

	ResetBruteForce(ip, req.Username)

	token, err := s.auth.GenerateToken(userID, req.Username, role)
	if err != nil {
		return echo.NewHTTPError(500, "token generation failed")
	}

	return c.JSON(http.StatusOK, map[string]string{
		"token": token,
	})
}

func (s *Server) handleAuthCallback(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "not implemented"})
}

func (s *Server) handleSwiftFlowWebhook(c echo.Context) error {
	var payload struct {
		UserID        string `json:"user_id"`
		DeviceID      string `json:"device_id"`
		IP            string `json:"ip_address"`
		MAC           string `json:"mac_address"`
		ClientType    string `json:"client_type"`
		BandwidthKbps int    `json:"bandwidth_kbps"`
		Token         string `json:"token"`
	}
	if err := c.Bind(&payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}

	sessionID := payload.DeviceID + "-" + payload.UserID
	if err := s.db.CreateSession(sessionID, payload.UserID, payload.DeviceID, payload.IP, payload.ClientType, payload.BandwidthKbps); err != nil {
		return echo.NewHTTPError(500, "session creation failed")
	}

	token, err := s.auth.GenerateToken(payload.UserID, payload.UserID, "user")
	if err != nil {
		return echo.NewHTTPError(500, "token generation failed")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"token":          token,
		"device_id":      payload.DeviceID,
		"bandwidth_kbps": payload.BandwidthKbps,
		"client_type":    payload.ClientType,
	})
}

func (s *Server) handleGetSession(c echo.Context) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(401, "unauthorized")
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"user_id":        user.UserID,
		"device_id":      user.DeviceID,
		"bandwidth_kbps": user.BandwidthKB,
		"timestamp":      time.Now().Unix(),
	})
}

func (s *Server) handleListUsers(c echo.Context) error {
	users, err := s.db.ListUsers()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")
	}
	return c.JSON(http.StatusOK, users)
}

func (s *Server) handleGetUser(c echo.Context) error {
	id := c.Param("id")
	user, err := s.db.GetUserByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	return c.JSON(http.StatusOK, user)
}

func (s *Server) handleCreateUser(c echo.Context) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	if req.Username == "" || req.Password == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "username and password required")
	}
	if req.Role == "" {
		req.Role = "user"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return echo.NewHTTPError(500, "password hashing failed")
	}
	id := uuid.New().String()
	if err := s.db.CreateUser(id, req.Username, string(hash), req.Role); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")
	}
	return c.JSON(http.StatusCreated, map[string]string{"id": id, "username": req.Username, "role": req.Role})
}

func (s *Server) handleUpdateUser(c echo.Context) error {
	id := c.Param("id")
	var req struct {
		Role string `json:"role"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	if err := s.db.UpdateUserRole(id, req.Role); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")
	}
	return c.JSON(http.StatusOK, map[string]string{"id": id, "role": req.Role})
}

func (s *Server) handleDeleteUser(c echo.Context) error {
	id := c.Param("id")
	if err := s.db.DeleteUser(id); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")
	}
	return c.NoContent(http.StatusNoContent)
}
