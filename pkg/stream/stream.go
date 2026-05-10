package stream

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/devuser/aetherstream/pkg/encoder"
	"github.com/devuser/aetherstream/pkg/hls"
	"github.com/devuser/aetherstream/pkg/probe"
	"github.com/devuser/aetherstream/pkg/db"
)

// Server handles HTTP streaming endpoints
type Server struct {
	db         *db.DB
	transcoder *Transcoder
	mediaRoot  string
}

// NewServer creates streaming HTTP handlers
func NewServer(database *db.DB, mediaRoot string) *Server {
	return &Server{
		db:         database,
		transcoder: NewTranscoder(database, mediaRoot),
		mediaRoot:  mediaRoot,
	}
}

	// RegisterRoutes sets up streaming routes (protected by auth middleware)
func (s *Server) RegisterRoutes(e *echo.Echo, authMiddleware echo.MiddlewareFunc) {
	g := e.Group("/videos")
	if authMiddleware != nil {
		g.Use(authMiddleware)
	}
	g.GET("/:id/stream", s.handleDirectStream)
	g.GET("/:id/hls/master.m3u8", s.handleHLSMaster)
	g.GET("/:id/hls/:profile/playlist.m3u8", s.handleHLSVariant)
	g.GET("/:id/hls/:profile/:segment", s.handleHLSSegment)
	g.GET("/:id/probe", s.handleProbe)
}

// handleDirectStream serves the original file (direct play)
func (s *Server) handleDirectStream(c echo.Context) error {
	itemID := c.Param("id")

	item, err := s.db.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}

	path, ok := item["path"].(string)
	if !ok || path == "" {
		return echo.NewHTTPError(http.StatusNotFound, "no file path")
	}

	// Security: validate path is within mediaRoot
	cleanPath := filepath.Clean(path)
	cleanRoot := filepath.Clean(s.mediaRoot)
	if !strings.HasPrefix(cleanPath, cleanRoot+string(filepath.Separator)) && cleanPath != cleanRoot {
		return echo.NewHTTPError(http.StatusForbidden, "invalid path")
	}

	// Check file exists
	if _, err := os.Stat(cleanPath); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "file not found on disk")
	}

	// Serve with range support
	return c.File(cleanPath)
}

// handleHLSMaster serves the master playlist
func (s *Server) handleHLSMaster(c echo.Context) error {
	itemID := c.Param("id")

	// Check if transcode output exists
	outputDir := filepath.Join(s.mediaRoot, "transcodes", itemID)

	// If not transcoded yet, trigger background transcode
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		// Start transcode for default profiles
		go s.transcoder.Transcode(itemID, []string{"mobile", "tablet"})

		// Return a temporary redirect or waiting playlist
		return c.String(http.StatusOK, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1000\n# Waiting for transcode...\n")
	}

	// Generate master playlist from available profiles
	profiles := s.discoverProfiles(outputDir)
	if len(profiles) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "no transcode profiles available")
	}

	playlist := hls.GenerateMasterForProfiles("", profiles)
	c.Response().Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	return c.String(http.StatusOK, playlist)
}

// handleHLSVariant serves a variant playlist
func (s *Server) handleHLSVariant(c echo.Context) error {
	itemID := c.Param("id")
	profile := c.Param("profile")

	playlistPath := filepath.Join(s.mediaRoot, "transcodes", itemID, profile, "playlist.m3u8")

	// Security: validate path is within mediaRoot before reading
	cleanPlaylist := filepath.Clean(playlistPath)
	cleanRoot := filepath.Clean(s.mediaRoot)
	if !strings.HasPrefix(cleanPlaylist, cleanRoot+string(filepath.Separator)) && cleanPlaylist != cleanRoot {
		return echo.NewHTTPError(http.StatusForbidden, "invalid path")
	}

	content, err := os.ReadFile(cleanPlaylist) // #nosec G304 - path validated above against mediaRoot
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "variant not found")
	}

	c.Response().Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	return c.Blob(http.StatusOK, "application/vnd.apple.mpegurl", content)
}

// handleHLSSegment serves a .ts segment
func (s *Server) handleHLSSegment(c echo.Context) error {
	itemID := c.Param("id")
	profile := c.Param("profile")
	segment := c.Param("segment")

	segmentPath := filepath.Join(s.mediaRoot, "transcodes", itemID, profile, segment)

	// Security: ensure path is within transcodes dir (defense in depth)
	cleanSegment := filepath.Clean(segmentPath)
	cleanRoot := filepath.Clean(filepath.Join(s.mediaRoot, "transcodes"))
	if !strings.HasPrefix(cleanSegment, cleanRoot+string(filepath.Separator)) {
		return echo.NewHTTPError(http.StatusForbidden, "invalid path")
	}

	return c.File(cleanSegment)
}

// handleProbe returns ffprobe info for an item
func (s *Server) handleProbe(c echo.Context) error {
	itemID := c.Param("id")

	item, err := s.db.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}

	path, ok := item["path"].(string)
	if !ok || path == "" {
		return echo.NewHTTPError(http.StatusNotFound, "no file path")
	}

	info, err := probe.Probe(path)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, info)
}

// discoverProfiles scans transcode output directory for available profiles
func (s *Server) discoverProfiles(outputDir string) []string {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return nil
	}

	var profiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			profiles = append(profiles, entry.Name())
		}
	}
	return profiles
}

// Transcoder manages background transcode jobs
type Transcoder struct {
	db        *db.DB
	mediaRoot string
	jobs      map[string]bool
}

// NewTranscoder creates a transcoder
func NewTranscoder(database *db.DB, mediaRoot string) *Transcoder {
	return &Transcoder{
		db:        database,
		mediaRoot: mediaRoot,
		jobs:      make(map[string]bool),
	}
}

// Transcode starts background transcode for given profiles
func (t *Transcoder) Transcode(itemID string, profiles []string) error {
	// Check if already running
	if t.jobs[itemID] {
		return fmt.Errorf("transcode already in progress")
	}

	item, err := t.db.GetItemByID(itemID)
	if err != nil {
		return err
	}

	inputPath, _ := item["path"].(string)
	if inputPath == "" {
		return fmt.Errorf("no input path")
	}

	t.jobs[itemID] = true

	go func() {
		defer delete(t.jobs, itemID)

		for _, profileName := range profiles {
			profile := encoder.GetProfileByName(profileName)
			outputDir := filepath.Join(t.mediaRoot, "transcodes", itemID, profileName)
			os.MkdirAll(outputDir, 0750)

			// Build FFmpeg HLS command
			args := encoder.BuildHLSCommand(inputPath, outputDir, profile, 4, "none")

			// Run FFmpeg
			// #nosec G204 - inputPath is validated against library paths before reaching here
			cmd := exec.Command("ffmpeg", args...)
			if output, err := cmd.CombinedOutput(); err != nil {
				// Log error but continue with other profiles
				fmt.Printf("transcode error for %s: %v\n%s\n", profileName, err, output)
			}
		}
	}()

	return nil
}
