package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/devuser/aetherstream/pkg/config"
	"github.com/devuser/aetherstream/pkg/api"
	"github.com/devuser/aetherstream/pkg/db"
	"github.com/devuser/aetherstream/pkg/auth"
	"github.com/devuser/aetherstream/pkg/dlna"
	"github.com/devuser/aetherstream/pkg/docs"
	"github.com/devuser/aetherstream/pkg/library"
	"github.com/devuser/aetherstream/pkg/metrics"
	"github.com/devuser/aetherstream/pkg/securestore"
)

func main() {
	// Logger setup
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Config
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	// Database
	database, err := db.New(cfg.Database.Path)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open database")
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		log.Fatal().Err(err).Msg("failed to run migrations")
	}

	if err := database.SeedAdminUser(); err != nil {
		log.Fatal().Err(err).Msg("failed to seed admin user")
	}

	// Auth
	authSvc, err := auth.NewService(cfg.Auth.Secret, cfg.Auth.TokenTTL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to init auth")
	}

	// Secure store for secrets
	secureStore, err := securestore.NewStoreFromEnv("AETHERSTREAM_MASTER_KEY")
	if err != nil {
		log.Warn().Err(err).Msg("secure store not initialized, using plaintext secrets")
		// Fallback: create with a derived key from auth secret (NOT for production)
		secureStore, _ = securestore.NewStore(cfg.Auth.Secret)
	}

	// Echo server
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.RequestID())
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(api.SecurityHeaders())
	// CORS: explicit origins only, no wildcard with credentials
	allowedOrigins := []string{"http://localhost:3000", "http://localhost:8081", "http://127.0.0.1:8081", "http://192.168.1.50:8081"}
	if len(cfg.Server.AllowedOrigins) > 0 {
		allowedOrigins = cfg.Server.AllowedOrigins
	}
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, echo.HeaderXRequestedWith},
		AllowCredentials: true,
		MaxAge:           86400,
	}))
	e.Use(middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Skipper: func(c echo.Context) bool {
			// Skip rate limiting for HLS/DASH/direct streaming endpoints —
			// Hls.js makes many rapid sequential segment requests and must not be throttled.
			return strings.HasPrefix(c.Request().URL.Path, "/videos/")
		},
		Store: middleware.NewRateLimiterMemoryStore(20),
	}))

	// Metrics + pprof
	m := metrics.NewMetrics()
	e.Use(m.EchoMiddleware())
	metrics.RegisterPProf(e)

	// Metrics server on port 9090
	go func() {
		metricsAddr := ":9090"
		if cfg.Server.Host != "" && cfg.Server.Host != "0.0.0.0" {
			metricsAddr = cfg.Server.Host + ":9090"
		}
		mux := http.NewServeMux()
		mux.Handle("/metrics", m.MetricsHandler())
		log.Info().Str("addr", metricsAddr).Msg("metrics server starting")
		server := &http.Server{
			Addr:         metricsAddr,
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}
		if err := server.ListenAndServe(); err != nil {
			log.Error().Err(err).Msg("metrics server crashed")
		}
	}()

	// Library manager
	libMgr, err := library.NewManager(database, cfg.SwiftFlow.APIKey) // reusing API key slot for TMDb
	if err != nil {
		log.Fatal().Err(err).Msg("failed to init library manager")
	}
	defer libMgr.Close()

	// API routes
	apiServer := api.NewServer(database, authSvc, cfg, libMgr, secureStore)
	apiServer.RegisterRoutes(e)

	// Swagger docs
	docs.RegisterRoutes(e)


	// DLNA/UPnP server
	dlnaServer := dlna.NewServer(database, cfg.Server.Host, cfg.Server.Port+1, "AetherStream")
	if err := dlnaServer.Start(); err != nil {
		log.Warn().Err(err).Msg("DLNA server failed to start")
	}
	defer dlnaServer.Stop()

	// Start
	go func() {
		addr := fmt.Sprintf("0.0.0.0:%d", cfg.Server.Port)
		log.Info().Str("addr", addr).Msg("AetherStream starting")
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server crashed")
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("shutdown error")
	}
}
