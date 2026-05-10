package api

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"

	"github.com/devuser/aetherstream/pkg/auth"
	"github.com/devuser/aetherstream/pkg/config"
	"github.com/devuser/aetherstream/pkg/db"
	"github.com/devuser/aetherstream/pkg/library"
	"github.com/devuser/aetherstream/pkg/stream"
)

// Server wraps Echo and dependencies
type Server struct {
	e       *echo.Echo
	db      *db.DB
	auth    *auth.Service
	cfg     *config.Config
	logger  zerolog.Logger
	library *library.Manager
}

// NewServer creates API server
func NewServer(database *db.DB, authSvc *auth.Service, cfg *config.Config, libMgr *library.Manager) *Server {
	return &Server{
		db:      database,
		auth:    authSvc,
		cfg:     cfg,
		logger:  zerolog.New(nil),
		library: libMgr,
	}
}

// RegisterRoutes sets up all API routes
func (s *Server) RegisterRoutes(e *echo.Echo) {
	s.e = e

	// Health / system
	e.GET("/system/info", s.handleSystemInfo)

	// Auth routes (public)
	e.POST("/auth/login", s.handleLogin)
	e.POST("/auth/callback", s.handleAuthCallback)
	e.POST("/webhooks/swiftflow", s.handleSwiftFlowWebhook)

	// Protected routes
	api := e.Group("/api")
	api.Use(s.auth.Middleware())

	// Users
	api.GET("/users", s.handleListUsers)
	api.GET("/users/:id", s.handleGetUser)

	// Items
	api.GET("/items", s.handleListItems)
	api.GET("/items/:id", s.handleGetItem)

	// Libraries
	api.GET("/libraries", s.handleListLibraries)
	api.POST("/libraries", s.handleCreateLibrary, auth.RequireRole("admin"))
	api.POST("/libraries/:id/scan", s.handleScanLibrary, auth.RequireRole("admin"))

	// Items
	api.GET("/items", s.handleListItems)
	api.GET("/items/:id", s.handleGetItem)

	// Stream routes
	streamSrv := stream.NewServer(s.db, "./media")
	streamSrv.RegisterRoutes(e)

	// Session info
	api.GET("/session", s.handleGetSession)
}

func (s *Server) handleSystemInfo(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"name":    "AetherStream",
		"version": "0.1.0",
		"status":  "ok",
	})
}

func (s *Server) handleLogin(c echo.Context) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(400, "invalid request")
	}

	// For Phase 1: simple demo login — admin/admin
	if req.Username != "admin" || req.Password != "admin" {
		return echo.NewHTTPError(401, "invalid credentials")
	}

	token, err := s.auth.GenerateToken("admin-1", "admin", "admin")
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

func (s *Server) handleListUsers(c echo.Context) error {
	return c.JSON(http.StatusOK, []map[string]string{})
}

func (s *Server) handleGetUser(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
}

func (s *Server) handleListItems(c echo.Context) error {
	return c.JSON(http.StatusOK, []map[string]string{})
}

func (s *Server) handleGetItem(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
}

func (s *Server) handleListLibraries(c echo.Context) error {
	return c.JSON(http.StatusOK, []map[string]string{})
}

func (s *Server) handleCreateLibrary(c echo.Context) error {
	var req struct {
		Name      string `json:"name"`
		Path      string `json:"path"`
		MediaType string `json:"media_type"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(400, "invalid request")
	}

	if req.Name == "" || req.Path == "" {
		return echo.NewHTTPError(400, "name and path required")
	}

	id, err := s.library.CreateLibrary(req.Name, req.Path, req.MediaType)
	if err != nil {
		return echo.NewHTTPError(500, err.Error())
	}

	return c.JSON(http.StatusCreated, map[string]string{
		"id":   id,
		"name": req.Name,
		"path": req.Path,
	})
}

func (s *Server) handleScanLibrary(c echo.Context) error {
	id := c.Param("id")
	if err := s.library.ScanLibrary(id); err != nil {
		return echo.NewHTTPError(500, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "scanning", "library_id": id})
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
		return echo.NewHTTPError(400, "invalid payload")
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
