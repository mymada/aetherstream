package integrations

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ArrWebhookPayload represents the common webhook payload from Sonarr/Radarr.
type ArrWebhookPayload struct {
	EventType string `json:"eventType"`
	Series    *struct {
		Title     string `json:"title"`
		Path      string `json:"path"`
		TVDBID    int    `json:"tvdbId"`
	}
	Movie     *struct {
		Title       string `json:"title"`
		Path        string `json:"folderPath"`
		TMDBID      int    `json:"tmdbId"`
	}
	Episodes  []struct {
		Title       string `json:"title"`
		SeasonNumber int   `json:"seasonNumber"`
		EpisodeNumber int  `json:"episodeNumber"`
		Path        string `json:"path"`
	} `json:"episodes"`
	EpisodeFile *struct {
		RelativePath string `json:"relativePath"`
		Path         string `json:"path"`
	} `json:"episodeFile"`
	MovieFile *struct {
		RelativePath string `json:"relativePath"`
		Path         string `json:"path"`
	} `json:"movieFile"`
	IsUpgrade bool   `json:"isUpgrade"`
}

// ArrConfig holds configuration for Sonarr/Radarr webhook endpoints.
type ArrConfig struct {
	SonarrSecret  string
	RadarrSecret  string
	MediaRoot     string // base path where media is stored (e.g., /media)
}

// ArrService handles webhook events from Sonarr/Radarr.
type ArrService struct {
	cfg ArrConfig
}

// NewArrService creates a new ArrService.
func NewArrService(cfg ArrConfig) *ArrService {
	return &ArrService{cfg: cfg}
}

// RegisterRoutes registers webhook endpoints on the Echo router.
func (s *ArrService) RegisterRoutes(e *echo.Echo) {
	e.POST("/webhooks/sonarr", s.handleSonarrWebhook)
	e.POST("/webhooks/radarr", s.handleRadarrWebhook)
}

// verifySignature checks the X-Webhook-Signature header against the secret.
func verifySignature(body []byte, secret, signature string) bool {
	if secret == "" || signature == "" {
		return true // no secret configured, accept all
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func (s *ArrService) handleSonarrWebhook(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot read body")
	}
	sig := c.Request().Header.Get("X-Webhook-Signature")
	if !verifySignature(body, s.cfg.SonarrSecret, sig) {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid signature")
	}

	var payload ArrWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid json")
	}

	log.Info().
		Str("event", payload.EventType).
		Str("service", "sonarr").
		Msg("received Sonarr webhook")

	// Process the event
	switch strings.ToLower(payload.EventType) {
	case "download", "episodefileadded":
		if err := s.processDownload(payload, "tv"); err != nil {
			log.Error().Err(err).Msg("sonarr download processing failed")
		}
	case "test":
		log.Info().Msg("sonarr test webhook received")
	default:
		log.Debug().Str("event", payload.EventType).Msg("unhandled sonarr event")
	}

	return c.NoContent(http.StatusNoContent)
}

func (s *ArrService) handleRadarrWebhook(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot read body")
	}
	sig := c.Request().Header.Get("X-Webhook-Signature")
	if !verifySignature(body, s.cfg.RadarrSecret, sig) {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid signature")
	}

	var payload ArrWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid json")
	}

	log.Info().
		Str("event", payload.EventType).
		Str("service", "radarr").
		Msg("received Radarr webhook")

	switch strings.ToLower(payload.EventType) {
	case "download", "moviefileadded":
		if err := s.processDownload(payload, "movie"); err != nil {
			log.Error().Err(err).Msg("radarr download processing failed")
		}
	case "test":
		log.Info().Msg("radarr test webhook received")
	default:
		log.Debug().Str("event", payload.EventType).Msg("unhandled radarr event")
	}

	return c.NoContent(http.StatusNoContent)
}

// processDownload handles the download event by logging and optionally triggering a library scan.
func (s *ArrService) processDownload(payload ArrWebhookPayload, mediaType string) error {
	var paths []string
	if mediaType == "tv" && payload.Series != nil {
		basePath := payload.Series.Path
		if basePath != "" {
			paths = append(paths, basePath)
		}
		if payload.EpisodeFile != nil && payload.EpisodeFile.Path != "" {
			paths = append(paths, payload.EpisodeFile.Path)
		}
		for _, ep := range payload.Episodes {
			if ep.Path != "" {
				paths = append(paths, ep.Path)
			}
		}
	}
	if mediaType == "movie" && payload.Movie != nil {
		basePath := payload.Movie.Path
		if basePath != "" {
			paths = append(paths, basePath)
		}
		if payload.MovieFile != nil && payload.MovieFile.Path != "" {
			paths = append(paths, payload.MovieFile.Path)
		}
	}

	for _, p := range paths {
		if p == "" {
			continue
		}
		// Normalize path under media root if possible
		cleanPath := filepath.Clean(p)
		if s.cfg.MediaRoot != "" {
			if strings.HasPrefix(cleanPath, s.cfg.MediaRoot) {
				// already under root
			} else {
				// attempt to join if relative
				cleanPath = filepath.Join(s.cfg.MediaRoot, cleanPath)
			}
		}
		// Validate path exists
		if _, err := os.Stat(cleanPath); err != nil {
			log.Warn().Str("path", cleanPath).Err(err).Msg("downloaded file path not accessible")
			continue
		}
		log.Info().
			Str("path", cleanPath).
			Str("type", mediaType).
			Msg("new media available from arr webhook")
	}
	return nil
}

// TriggerLibraryScan is a placeholder to notify the library manager to rescan a path.
// In a real implementation this would call the library manager directly.
func (s *ArrService) TriggerLibraryScan(path string) error {
	log.Info().Str("path", path).Msg("triggering library scan")
	return nil
}

// HealthCheck returns a simple health status for the integration.
func (s *ArrService) HealthCheck() map[string]interface{} {
	return map[string]interface{}{
		"sonarr_webhook": s.cfg.SonarrSecret != "",
		"radarr_webhook": s.cfg.RadarrSecret != "",
		"media_root":     s.cfg.MediaRoot,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	}
}
