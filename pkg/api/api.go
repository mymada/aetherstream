package api

import (
	"path/filepath"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"

	"github.com/devuser/aetherstream/pkg/auth"
	"github.com/devuser/aetherstream/pkg/config"
	"github.com/devuser/aetherstream/pkg/db"
	"github.com/devuser/aetherstream/pkg/library"
	"github.com/devuser/aetherstream/pkg/playback"
	"github.com/devuser/aetherstream/pkg/search"
	"github.com/devuser/aetherstream/pkg/securestore"
	"github.com/devuser/aetherstream/pkg/stream"
	"github.com/devuser/aetherstream/pkg/tasks"
	"github.com/devuser/aetherstream/pkg/thumbnail"
	"github.com/devuser/aetherstream/pkg/ws"
)

type Server struct {
	e            *echo.Echo
	db           *db.DB
	auth         *auth.Service
	cfg          *config.Config
	logger       zerolog.Logger
	library      *library.Manager
	secureStore  *securestore.Store
	thumbSvc     *thumbnail.Service
	searcher     *search.Searcher
	StreamServer *stream.Server
}

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

func (s *Server) RegisterRoutes(e *echo.Echo) {
	s.e = e

	e.Use(SecurityHeaders())
	e.Use(SecureCookieMiddleware())
	e.Use(CSRFProtection(s.cfg))
	e.Use(BruteForceProtection())
	e.Use(CORSMiddleware(s.cfg))

	e.GET("/system/info", s.handleSystemInfo, RateLimitByIP(1000))
	e.GET("/api/system/hardware", s.handleSystemHardware, RateLimitByIP(1000))
	e.GET("/health", s.handleHealth)
	e.GET("/ready", s.handleReady)

	webUIPath := s.cfg.Server.WebUIPath
	if webUIPath == "" {
		webUIPath = "web/dist"
	}
	e.Static("/app/assets", webUIPath+"/assets")
	e.File("/app", webUIPath+"/index.html")
	e.File("/vite.svg", webUIPath+"/vite.svg")
	e.GET("/app/*", func(c echo.Context) error {
		return c.File(webUIPath + "/index.html")
	})

	e.GET("/", func(c echo.Context) error {
		return c.Redirect(301, "/app")
	})

	e.GET("/tv", func(c echo.Context) error {
		return c.Redirect(301, "/app/tv")
	})
	e.GET("/tv/*", func(c echo.Context) error {
		return c.Redirect(301, "/app/tv/"+c.Param("*"))
	})

	e.GET("/login", func(c echo.Context) error {
		return c.Redirect(301, "/app/login")
	})
	e.GET("/register", func(c echo.Context) error {
		return c.Redirect(301, "/app/register")
	})
	e.GET("/libraries", func(c echo.Context) error {
		return c.Redirect(301, "/app/libraries")
	})
	e.GET("/libraries/*", func(c echo.Context) error {
		return c.Redirect(301, "/app/libraries/"+c.Param("*"))
	})
	e.GET("/player/*", func(c echo.Context) error {
		return c.Redirect(301, "/app/player/"+c.Param("*"))
	})
	e.GET("/settings", func(c echo.Context) error {
		return c.Redirect(301, "/app/settings")
	})

	e.POST("/auth/login", s.handleLogin, RateLimitByIP(10))
	e.POST("/auth/register", s.handleRegister, RateLimitByIP(10))
	e.POST("/auth/callback", s.handleAuthCallback, RateLimitByIP(10))
	e.POST("/webhooks/swiftflow", s.handleSwiftFlowWebhook, RateLimitByIP(100))

	api := e.Group("/api")
	api.Use(s.auth.Middleware())
	api.Use(SessionTimeout(30*time.Minute, s.db))

	api.GET("/users", s.handleListUsers)
	api.GET("/users/:id", s.handleGetUser)

	api.GET("/libraries", s.handleListLibraries)
	api.POST("/libraries", s.handleCreateLibrary, auth.RequireRole("admin"))
	api.POST("/libraries/:id/scan", s.handleScanLibrary, auth.RequireRole("admin"))

	api.GET("/items", s.handleListItems)
	api.GET("/items/:id", s.handleGetItem)
	api.GET("/items/:id/subtitles", s.handleListSubtitles)
	api.GET("/items/:id/subtitles/:lang", s.handleGetSubtitle)
	api.GET("/items/:id/thumbnails/:type", s.handleGetThumbnail)
	api.GET("/items/:id/chapters", s.handleListChapters)
	api.GET("/items/:id/chapters/at", s.handleGetChapterAt)
	api.POST("/items/:id/chapters/scan", s.handleScanChapters, auth.RequireRole("admin"))
	api.GET("/items/:id/trickplay", s.handleTrickplay)
	api.GET("/continue-watching", s.handleContinueWatching)
	api.POST("/items/:id/progress", s.handleSaveProgress)
	api.GET("/items/:id/progress", s.handleGetProgress)
	api.POST("/items/:id/watched", s.handleMarkWatched)

	api.GET("/audit-logs", s.handleAuditLogs, auth.RequireRole("admin"))
	api.POST("/auth/2fa/enable", s.handleEnable2FA)
	api.POST("/auth/2fa/verify", s.handleVerify2FA)
	api.GET("/api-keys", s.handleAPIKeys)

	api.GET("/users/:id/playback-reporting", s.handlePlaybackReporting)

	api.GET("/search", s.handleSearch)
	api.GET("/settings", s.handleGetSettings, auth.RequireRole("admin"))
	api.PUT("/settings", s.handleUpdateSettings, auth.RequireRole("admin"))

	api.POST("/users", s.handleCreateUser, auth.RequireRole("admin"))
	api.PUT("/users/:id", s.handleUpdateUser, auth.RequireRole("admin"))
	api.DELETE("/users/:id", s.handleDeleteUser, auth.RequireRole("admin"))

	api.GET("/collections", s.handleListCollections)
	api.POST("/collections", s.handleCreateCollection)
	api.GET("/collections/:id", s.handleGetCollection)
	api.POST("/collections/:id/items", s.handleAddToCollection)
	api.DELETE("/collections/:id/items/:item_id", s.handleRemoveFromCollection)

	api.GET("/activity", s.handleListActivity)

	api.GET("/jobs", s.handleListJobs)
	api.DELETE("/jobs/:id", s.handleCancelJob)
	api.GET("/transcodes", s.handleListTranscodes, auth.RequireRole("admin"))
	api.DELETE("/transcodes/:key", s.handleDeleteTranscode, auth.RequireRole("admin"))

	e.GET("/ws", s.handleWebSocket, s.auth.Middleware())

	e.GET("/ws/playback", s.handlePlaybackWebSocket)

	playbackStore := playback.NewStore(s.db)
	_ = playbackStore.Migrate()
	playbackHandler := playback.NewHandler(playbackStore, func(event playback.PlaybackEvent) {
		ws.ForwardPlaybackEvent(event)
	})
	playbackHandler.RegisterRoutes(api)

	mediaRoot := s.cfg.Server.MediaRoot
	if mediaRoot == "" {
		mediaRoot = s.cfg.Server.StaticPath
	}
	if mediaRoot == "" {
		mediaRoot = "./media"
	}
	streamSrv := stream.NewServer(s.db, mediaRoot)
	s.StreamServer = streamSrv
	streamSrv.RegisterRoutes(e, s.auth.Middleware())
	stream.RegisterAdaptiveRoutes(e, s.db, mediaRoot, s.auth.Middleware())

	tasks.StartCleanupLoop(filepath.Join(mediaRoot, "transcodes"), tasks.DefaultCleanupConfig(), streamSrv.JobManager())

	api.GET("/session", s.handleGetSession)

	// Filesystem browser (admin only — same as Jellyfin /Environment/*)
	api.GET("/environment/drives", s.handleGetDrives, auth.RequireRole("admin"))
	api.GET("/environment/directory", s.handleGetDirectoryContents, auth.RequireRole("admin"))
	api.GET("/environment/parent", s.handleGetParentPath, auth.RequireRole("admin"))
}
