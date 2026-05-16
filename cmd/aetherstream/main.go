package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/devuser/aetherstream/pkg/admin"
	"github.com/devuser/aetherstream/pkg/api"
	"github.com/devuser/aetherstream/pkg/auth"
	"github.com/devuser/aetherstream/pkg/config"
	"github.com/devuser/aetherstream/pkg/db"
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

	if err := database.MigrateProfiles(); err != nil {
		log.Fatal().Err(err).Msg("failed to run profile migrations")
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
		if isProduction() {
			log.Fatal().Err(err).Msg("AETHERSTREAM_MASTER_KEY is required in production")
		}
		log.Warn().Err(err).Msg("AETHERSTREAM_MASTER_KEY not set; using ephemeral development secure store")
		secureStore, err = securestore.NewStore(generateEphemeralSecret(32))
		if err != nil {
			log.Fatal().Err(err).Msg("failed to initialize development secure store")
		}
	}

	// Echo server
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.RequestID())
	e.Use(requestLogger())
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
		metricsAddr := fmt.Sprintf(":%d", cfg.Server.MetricsPort)
		if cfg.Server.Host != "" && cfg.Server.Host != "0.0.0.0" {
			metricsAddr = fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.MetricsPort)
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
			if err == http.ErrServerClosed {
				return
			}
			log.Warn().Err(err).Msg("metrics server unavailable")
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

	// Account self-management routes (each user manages their own account)
	accountRoutes := api.NewAccountRoutes(database)
	accountRoutes.RegisterRoutes(e, authSvc.Middleware())

	// Profile routes (user-managed profiles)
	profileRoutes := api.NewProfileRoutes(database, authSvc)
	profileRoutes.RegisterRoutes(e, authSvc.Middleware())

	// Admin routes (require admin role - single admin only)
	adminServer := admin.NewServer(database)
	adminServer.RegisterRoutes(e, authSvc.Middleware(), api.AdminMiddleware())

	// Cast / AirPlay / WebRTC routes
	castRoutes := api.NewCastRoutes(cfg, database)
	castRoutes.Start()
	castRoutes.RegisterRoutes(e, authSvc.Middleware())

	// Castv2 native routes (bypasses browser SDK restrictions)
	castv2Routes := api.NewCastv2Routes(cfg, database)
	castv2Routes.Start()
	castv2Routes.RegisterRoutes(e, authSvc.Middleware())

	// Swagger docs
	docs.RegisterRoutes(e)

	// DLNA/UPnP server
	dlnaServer := dlna.NewServer(database, cfg.Server.Host, cfg.Server.DLNAPort, "AetherStream")
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

	// Cancel all active transcode jobs (graceful FFmpeg shutdown)
	if apiServer.StreamServer != nil {
		for _, j := range apiServer.StreamServer.JobManager().List() {
			_ = apiServer.StreamServer.JobManager().Cancel(j.ID)
		}
		log.Info().Msg("all active sessions cancelled")
	}

	castRoutes.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("shutdown error")
	}
}

func isProduction() bool {
	for _, key := range []string{"AETHERSTREAM_ENV", "APP_ENV", "GO_ENV"} {
		if strings.EqualFold(os.Getenv(key), "production") || strings.EqualFold(os.Getenv(key), "prod") {
			return true
		}
	}
	return false
}

func generateEphemeralSecret(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		log.Fatal().Err(err).Msg("failed to generate ephemeral secure store key")
	}
	return hex.EncodeToString(bytes)
}

func requestLogger() echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogLatency:       true,
		LogRemoteIP:      true,
		LogHost:          true,
		LogMethod:        true,
		LogURI:           true,
		LogRequestID:     true,
		LogUserAgent:     true,
		LogStatus:        true,
		LogError:         true,
		LogContentLength: true,
		LogResponseSize:  true,
		HandleError:      true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			ev := log.Info()
			if v.Error != nil || v.Status >= http.StatusInternalServerError {
				ev = log.Error()
			} else if v.Status >= http.StatusBadRequest {
				ev = log.Warn()
			}
			if v.Error != nil {
				ev = ev.Err(v.Error)
			}
			ev.
				Str("id", v.RequestID).
				Str("remote_ip", v.RemoteIP).
				Str("host", v.Host).
				Str("method", v.Method).
				Str("uri", sanitizeURI(v.URI)).
				Str("user_agent", v.UserAgent).
				Int("status", v.Status).
				Dur("latency", v.Latency).
				Str("bytes_in", v.ContentLength).
				Int64("bytes_out", v.ResponseSize).
				Msg("request")
			return nil
		},
	})
}

func sanitizeURI(raw string) string {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return redactQuerySecrets(raw)
	}
	q := u.Query()
	for _, key := range []string{"token", "access_token", "refresh_token", "id_token", "api_key", "apikey", "key", "secret", "password"} {
		if _, ok := q[key]; ok {
			q.Set(key, "[REDACTED]")
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func redactQuerySecrets(raw string) string {
	for _, key := range []string{"token", "access_token", "refresh_token", "id_token", "api_key", "apikey", "key", "secret", "password"} {
		marker := key + "="
		if idx := strings.Index(raw, marker); idx >= 0 {
			start := idx + len(marker)
			end := start
			for end < len(raw) && raw[end] != '&' && raw[end] != ' ' {
				end++
			}
			raw = raw[:start] + "[REDACTED]" + raw[end:]
		}
	}
	return raw
}
