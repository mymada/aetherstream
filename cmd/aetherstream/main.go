package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
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
	"github.com/devuser/aetherstream/pkg/library"
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
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:8080"},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		AllowCredentials: true,
		MaxAge:           86400,
	}))
	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(20)))

	// Library manager
	libMgr, err := library.NewManager(database, cfg.SwiftFlow.APIKey) // reusing API key slot for TMDb
	if err != nil {
		log.Fatal().Err(err).Msg("failed to init library manager")
	}
	defer libMgr.Close()

	// API routes
	apiServer := api.NewServer(database, authSvc, cfg, libMgr, secureStore)
	apiServer.RegisterRoutes(e)

	// Start
	go func() {
		addr := fmt.Sprintf(":%d", cfg.Server.Port)
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
