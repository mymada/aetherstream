package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"

	"github.com/devuser/aetherstream/pkg/auth"
	"github.com/devuser/aetherstream/pkg/config"
	"github.com/devuser/aetherstream/pkg/db"
	"github.com/devuser/aetherstream/pkg/encoder"
	"github.com/devuser/aetherstream/pkg/library"
	"github.com/devuser/aetherstream/pkg/probe"
	"github.com/devuser/aetherstream/pkg/search"
	"github.com/devuser/aetherstream/pkg/securestore"
	"github.com/devuser/aetherstream/pkg/stream"
	"github.com/devuser/aetherstream/pkg/thumbnail"
	"github.com/devuser/aetherstream/pkg/ws"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Server wraps Echo and dependencies
type Server struct {
	e           *echo.Echo
	db          *db.DB
	auth        *auth.Service
	cfg         *config.Config
	logger      zerolog.Logger
	library     *library.Manager
	secureStore *securestore.Store
	thumbSvc    *thumbnail.Service
	searcher    *search.Searcher
}

// NewServer creates API server
func NewServer(database *db.DB, authSvc *auth.Service, cfg *config.Config, libMgr *library.Manager, store *securestore.Store) *Server {
	return &Server{
		db:          database,
		auth:        authSvc,
		cfg:         cfg,
		logger:      zerolog.New(nil),
		library:     libMgr,
		secureStore: store,
		thumbSvc:    thumbnail.NewService(cfg.FFmpeg.Path, "./thumbnails"),
		searcher:    search.NewSearcher(database),
	}
}

// RegisterRoutes sets up all API routes
func (s *Server) RegisterRoutes(e *echo.Echo) {
	s.e = e

	// Health / system
	e.GET("/system/info", s.handleSystemInfo, RateLimitByIP(1000))
	e.GET("/api/system/hardware", s.handleSystemHardware, RateLimitByIP(1000))

	// Auth routes (public)
	e.POST("/auth/login", s.handleLogin, RateLimitByIP(10))
	e.POST("/auth/callback", s.handleAuthCallback, RateLimitByIP(10))
	e.POST("/webhooks/swiftflow", s.handleSwiftFlowWebhook, RateLimitByIP(100))

	// Protected routes
	api := e.Group("/api")
	api.Use(s.auth.Middleware())

	// Users
	api.GET("/users", s.handleListUsers)
	api.GET("/users/:id", s.handleGetUser)

	// Libraries
	api.GET("/libraries", s.handleListLibraries)
	api.POST("/libraries", s.handleCreateLibrary, auth.RequireRole("admin"))
	api.POST("/libraries/:id/scan", s.handleScanLibrary, auth.RequireRole("admin"))

	// Items
	api.GET("/items", s.handleListItems)
	api.GET("/items/:id", s.handleGetItem)
	api.GET("/items/:id/subtitles", s.handleListSubtitles)
	api.GET("/items/:id/subtitles/:lang", s.handleGetSubtitle)
	api.GET("/items/:id/thumbnails/:type", s.handleGetThumbnail)

	// Search
	api.GET("/search", s.handleSearch)

	// Users (admin only for write)
	api.POST("/users", s.handleCreateUser, auth.RequireRole("admin"))
	api.PUT("/users/:id", s.handleUpdateUser, auth.RequireRole("admin"))
	api.DELETE("/users/:id", s.handleDeleteUser, auth.RequireRole("admin"))

	// Collections / Playlists
	api.GET("/collections", s.handleListCollections)
	api.POST("/collections", s.handleCreateCollection)
	api.GET("/collections/:id", s.handleGetCollection)
	api.POST("/collections/:id/items", s.handleAddToCollection)
	api.DELETE("/collections/:id/items/:item_id", s.handleRemoveFromCollection)

	// Dashboard / Activity
	api.GET("/activity", s.handleListActivity)

	// WebSocket realtime
	e.GET("/ws", s.handleWebSocket)

	// Stream routes
	streamSrv := stream.NewServer(s.db, "./media")
	streamSrv.RegisterRoutes(e)
	stream.RegisterAdaptiveRoutes(e, s.db, "./media")

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

func (s *Server) handleSystemHardware(c echo.Context) error {
	caps := encoder.DetectHardwareCapabilities()
	return c.JSON(http.StatusOK, caps)
}

func (s *Server) handleLogin(c echo.Context) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(400, "invalid request")
	}

	// Lookup user from DB
	_, passwordHash, _, err := s.db.GetUserByUsername(req.Username)
	if err != nil {
		return echo.NewHTTPError(401, "invalid credentials")
	}

	// Verify password with bcrypt
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		return echo.NewHTTPError(401, "invalid credentials")
	}

	// Generate JWT token with actual user info
	token, err := s.auth.GenerateToken("admin-1", req.Username, "admin")
	if err != nil {
		return echo.NewHTTPError(500, "token generation failed")
	}

	return c.JSON(http.StatusOK, map[string]string{
		"token": token,
	})
}

// handleAuthCallback is a placeholder for OAuth callback
func (s *Server) handleAuthCallback(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "not implemented"})
}
func (s *Server) handleListUsers(c echo.Context) error {
	users, err := s.db.ListUsers()
	if err != nil {
		return echo.NewHTTPError(500, err.Error())
	}
	return c.JSON(http.StatusOK, users)
}

func (s *Server) handleGetUser(c echo.Context) error {
	id := c.Param("id")
	users, err := s.db.ListUsers()
	if err != nil {
		return echo.NewHTTPError(500, err.Error())
	}
	for _, u := range users {
		if uid, ok := u["id"].(string); ok && uid == id {
			return c.JSON(http.StatusOK, u)
		}
	}
	return echo.NewHTTPError(http.StatusNotFound, "user not found")
}

func (s *Server) handleListItems(c echo.Context) error {
	libID := c.QueryParam("library_id")
	if libID == "" {
		return echo.NewHTTPError(400, "library_id required")
	}
	items, err := s.db.ListItemsByLibrary(libID)
	if err != nil {
		return echo.NewHTTPError(500, err.Error())
	}
	return c.JSON(http.StatusOK, items)
}

func (s *Server) handleGetItem(c echo.Context) error {
	id := c.Param("id")
	item, err := s.db.GetItemByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}
	return c.JSON(http.StatusOK, item)
}

func (s *Server) handleListLibraries(c echo.Context) error {
	libs, err := s.db.ListLibraries()
	if err != nil {
		return echo.NewHTTPError(500, err.Error())
	}
	return c.JSON(http.StatusOK, libs)
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

// --- Subtitles ---

func (s *Server) handleListSubtitles(c echo.Context) error {
	itemID := c.Param("id")
	item, err := s.db.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}
	path, _ := item["path"].(string)
	subs, err := probe.ExtractSubtitleTracks(path)
	if err != nil {
		return c.JSON(http.StatusOK, []map[string]interface{}{})
	}
	return c.JSON(http.StatusOK, subs)
}

func (s *Server) handleGetSubtitle(c echo.Context) error {
	itemID := c.Param("id")
	lang := c.Param("lang")
	item, err := s.db.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}
	path, _ := item["path"].(string)
	// Extract subtitle to temp file and serve
	subPath, err := probe.ExtractSubtitleToFile(path, lang)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "subtitle not found")
	}
	return c.File(subPath)
}

// --- Users CRUD ---

func (s *Server) handleCreateUser(c echo.Context) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(400, "invalid request")
	}
	if req.Username == "" || req.Password == "" {
		return echo.NewHTTPError(400, "username and password required")
	}
	if req.Role == "" {
		req.Role = "user"
	}
	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return echo.NewHTTPError(500, "password hashing failed")
	}
	id := uuid.New().String()
	if err := s.db.CreateUser(id, req.Username, string(hash), req.Role); err != nil {
		return echo.NewHTTPError(500, err.Error())
	}
	return c.JSON(http.StatusCreated, map[string]string{"id": id, "username": req.Username, "role": req.Role})
}

func (s *Server) handleUpdateUser(c echo.Context) error {
	id := c.Param("id")
	var req struct {
		Role string `json:"role"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(400, "invalid request")
	}
	if err := s.db.UpdateUserRole(id, req.Role); err != nil {
		return echo.NewHTTPError(500, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]string{"id": id, "role": req.Role})
}

func (s *Server) handleDeleteUser(c echo.Context) error {
	id := c.Param("id")
	if err := s.db.DeleteUser(id); err != nil {
		return echo.NewHTTPError(500, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

// --- Collections ---

func (s *Server) handleListCollections(c echo.Context) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(401, "unauthorized")
	}
	cols, err := s.db.ListCollections(user.UserID)
	if err != nil {
		return echo.NewHTTPError(500, err.Error())
	}
	return c.JSON(http.StatusOK, cols)
}

func (s *Server) handleCreateCollection(c echo.Context) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(401, "unauthorized")
	}
	var req struct {
		Name string `json:"name"`
		Type string `json:"type"` // "collection" or "playlist"
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(400, "invalid request")
	}
	if req.Name == "" {
		return echo.NewHTTPError(400, "name required")
	}
	if req.Type == "" {
		req.Type = "collection"
	}
	id := uuid.New().String()
	if err := s.db.CreateCollection(id, user.UserID, req.Name, req.Type); err != nil {
		return echo.NewHTTPError(500, err.Error())
	}
	return c.JSON(http.StatusCreated, map[string]string{"id": id, "name": req.Name, "type": req.Type})
}

func (s *Server) handleGetCollection(c echo.Context) error {
	id := c.Param("id")
	col, items, err := s.db.GetCollectionWithItems(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "collection not found")
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"collection": col,
		"items":      items,
	})
}

func (s *Server) handleAddToCollection(c echo.Context) error {
	colID := c.Param("id")
	var req struct {
		ItemID string `json:"item_id"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(400, "invalid request")
	}
	if err := s.db.AddItemToCollection(colID, req.ItemID); err != nil {
		return echo.NewHTTPError(500, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Server) handleRemoveFromCollection(c echo.Context) error {
	colID := c.Param("id")
	itemID := c.Param("item_id")
	if err := s.db.RemoveItemFromCollection(colID, itemID); err != nil {
		return echo.NewHTTPError(500, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

// --- Search ---

func (s *Server) handleSearch(c echo.Context) error {
	q := c.QueryParam("q")
	if q == "" {
		return echo.NewHTTPError(400, "query parameter 'q' required")
	}
	mediaType := c.QueryParam("type")
	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if _, err := fmt.Sscanf(l, "%d", &limit); err != nil {
			limit = 20
		}
	}
	results, err := s.searcher.SearchItems(q, mediaType, limit)
	if err != nil {
		return echo.NewHTTPError(500, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"query":   q,
		"type":    mediaType,
		"limit":   limit,
		"results": results,
	})
}

// --- Activity / Dashboard ---

func (s *Server) handleListActivity(c echo.Context) error {
	acts, err := s.db.ListActivity(50)
	if err != nil {
		return echo.NewHTTPError(500, err.Error())
	}
	return c.JSON(http.StatusOK, acts)
}

// --- WebSocket ---

func (s *Server) handleWebSocket(c echo.Context) error {
	return ws.HandleWebSocket(c, s.db)
}

// --- Thumbnails ---

func (s *Server) handleGetThumbnail(c echo.Context) error {
	itemID := c.Param("id")
	thumbType := c.Param("type")

	item, err := s.db.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}

	path, ok := item["path"].(string)
	if !ok || path == "" {
		return echo.NewHTTPError(http.StatusNotFound, "no file path")
	}

	var t thumbnail.ThumbnailType
	switch thumbType {
	case "poster":
		t = thumbnail.TypePoster
	case "backdrop":
		t = thumbnail.TypeBackdrop
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "invalid thumbnail type")
	}

	// Generate on demand if missing
	if !s.thumbSvc.Exists(itemID, t) {
		_, _, err = s.thumbSvc.GenerateThumbnails(path, itemID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "thumbnail generation failed")
		}
	}

	thumbPath := s.thumbSvc.Path(itemID, t)
	return c.File(thumbPath)
}
