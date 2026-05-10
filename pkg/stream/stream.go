package stream

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/devuser/aetherstream/pkg/db"
	"github.com/devuser/aetherstream/pkg/dash"
	"github.com/devuser/aetherstream/pkg/encoder"
	"github.com/devuser/aetherstream/pkg/hls"
	"github.com/devuser/aetherstream/pkg/probe"
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
	if authMiddleware == nil {
		panic("stream.RegisterRoutes: authMiddleware is required and cannot be nil")
	}
	g := e.Group("/videos")
	g.Use(authMiddleware)
	g.GET("/:id/stream", s.handleDirectStream)
	g.GET("/:id/hls/master.m3u8", s.handleHLSMaster)
	g.GET("/:id/hls/:profile/playlist.m3u8", s.handleHLSVariant)
	g.GET("/:id/hls/:profile/:segment", s.handleHLSSegment)
	g.GET("/:id/dash/manifest.mpd", s.handleDASHManifest)
	g.GET("/:id/dash/:profile/:file", s.handleDASHSegment)
	g.GET("/:id/probe", s.handleProbe)
	g.POST("/:id/burnin", s.handleBurnIn)
	g.GET("/:id/subtitles", s.handleListSubtitles)
	g.GET("/:id/subtitles/:lang/vtt", s.handleWebVTT)
}

// handleDirectStream serves the original file (direct play)
func (s *Server) handleDirectStream(c echo.Context) error {
	itemID := c.Param("id")

	item, err := s.db.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}

	path := item.Path
	if path == "" {
		return echo.NewHTTPError(http.StatusNotFound, "no file path")
	}

	// Security: validate path is within mediaRoot
	cleanPath := filepath.Clean(path)
	cleanRoot := filepath.Clean(s.mediaRoot)
	// If mediaRoot is relative, resolve it to absolute
	if !filepath.IsAbs(cleanRoot) {
		absRoot, err := filepath.Abs(cleanRoot)
		if err == nil {
			cleanRoot = absRoot
		}
	}
	if !filepath.IsAbs(cleanPath) {
		absPath, err := filepath.Abs(cleanPath)
		if err == nil {
			cleanPath = absPath
		}
	}
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

	path := item.Path
	if path == "" {
		return echo.NewHTTPError(http.StatusNotFound, "no file path")
	}

	info, err := probe.Probe(path)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, info)
}

// handleDASHManifest serves the DASH MPD manifest
func (s *Server) handleDASHManifest(c echo.Context) error {
	itemID := c.Param("id")

	item, err := s.db.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}

	outputDir := filepath.Join(s.mediaRoot, "transcodes", itemID)

	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		go s.transcoder.Transcode(itemID, []string{"mobile", "tablet"})
		return c.String(http.StatusOK, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<MPD xmlns=\"urn:mpeg:dash:schema:mpd:2011\" type=\"static\" profiles=\"urn:mpeg:dash:profile:isoff-on-demand:2011\" mediaPresentationDuration=\"PT0S\"><Period id=\"1\"><AdaptationSet id=\"waiting\" contentType=\"text\"><Representation id=\"waiting\" bandwidth=\"1000\"><BaseURL># Waiting for transcode...</BaseURL></Representation></AdaptationSet></Period></MPD>")
	}

	profiles := s.discoverProfiles(outputDir)
	if len(profiles) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "no transcode profiles available")
	}

	var videoReps []dash.RepresentationInfo
	var audioReps []dash.RepresentationInfo

	for _, profileName := range profiles {
		profile := dash.GetProfileByName(profileName)
		profile.BaseURL = profileName + "/"
		if profile.SegmentTemplate != nil {
			profile.SegmentTemplate.Initialization = profileName + "/" + profile.SegmentTemplate.Initialization
			profile.SegmentTemplate.MediaPattern = profileName + "/" + profile.SegmentTemplate.MediaPattern
		}

		if strings.Contains(profile.MimeType, "audio") {
			audioReps = append(audioReps, profile)
		} else {
			videoReps = append(videoReps, profile)
		}
	}

	mpd, err := dash.BuildMPDForItem(itemID, item.DurationSeconds, "", videoReps)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate manifest")
	}

	if len(audioReps) > 0 && len(mpd.Periods) > 0 {
		dash.AddAudioAdaptationSet(&mpd.Periods[0], "und", audioReps)
	}

	manifestXML, err := mpd.ToXML()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to serialize manifest")
	}

	c.Response().Header().Set("Content-Type", "application/dash+xml")
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")
	return c.String(http.StatusOK, manifestXML)
}

// handleDASHSegment serves DASH segments
func (s *Server) handleDASHSegment(c echo.Context) error {
	itemID := c.Param("id")
	profile := c.Param("profile")
	file := c.Param("file")

	if strings.Contains(file, "..") || strings.Contains(file, "/") || strings.Contains(file, "\\") {
		return echo.NewHTTPError(http.StatusForbidden, "invalid file name")
	}

	segmentPath := filepath.Join(s.mediaRoot, "transcodes", itemID, profile, file)

	cleanSegment := filepath.Clean(segmentPath)
	cleanRoot := filepath.Clean(filepath.Join(s.mediaRoot, "transcodes"))
	if !strings.HasPrefix(cleanSegment, cleanRoot+string(filepath.Separator)) {
		return echo.NewHTTPError(http.StatusForbidden, "invalid path")
	}

	if _, err := os.Stat(cleanSegment); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "segment not found")
	}

	contentType := "video/mp2t"
	c.Response().Header().Set("Content-Type", contentType)
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")
	return c.File(cleanSegment)
}

// handleBurnIn triggers subtitle burn-in for an item
func (s *Server) handleBurnIn(c echo.Context) error {
	itemID := c.Param("id")

	item, err := s.db.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}

	var req BurnInRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	req.ItemID = itemID

	if err := ValidateBurnInRequest(req, s.mediaRoot); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if req.Profile == "" {
		req.Profile = "original"
	}

	result, err := BurnIn(item.Path, req.Language, s.mediaRoot, req.Profile, req.HWAccel)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, result)
}

// handleListSubtitles returns all subtitle tracks for a video item
func (s *Server) handleListSubtitles(c echo.Context) error {
	itemID := c.Param("id")
	item, err := s.db.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}
	tracks, err := probe.ExtractSubtitleTracks(item.Path)
	if err != nil {
		return c.JSON(http.StatusOK, []map[string]interface{}{})
	}
	return c.JSON(http.StatusOK, tracks)
}

// handleWebVTT serves a WebVTT subtitle for a specific language
func (s *Server) handleWebVTT(c echo.Context) error {
	itemID := c.Param("id")
	lang := c.Param("lang")

	item, err := s.db.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}

	if strings.Contains(lang, "..") || strings.Contains(lang, "/") || strings.Contains(lang, "\\") {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid language parameter")
	}

	subPath, err := probe.ExtractSubtitleToFile(item.Path, lang)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "subtitle not found")
	}
	if !strings.HasPrefix(subPath, os.TempDir()) && !strings.HasPrefix(subPath, "./thumbnails") {
		return echo.NewHTTPError(http.StatusInternalServerError, "invalid subtitle path")
	}
	return c.File(subPath)
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
	mu        sync.RWMutex
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
	t.mu.RLock()
	running := t.jobs[itemID]
	t.mu.RUnlock()
	if running {
		return fmt.Errorf("transcode already in progress")
	}

	item, err := t.db.GetItemByID(itemID)
	if err != nil {
		return err
	}

	inputPath := item.Path
	if inputPath == "" {
		return fmt.Errorf("no input path")
	}

	t.mu.Lock()
	t.jobs[itemID] = true
	t.mu.Unlock()

	go func() {
		defer func() {
			t.mu.Lock()
			delete(t.jobs, itemID)
			t.mu.Unlock()
		}()

		for _, profileName := range profiles {
			profile := encoder.GetProfileByName(profileName)
			outputDir := filepath.Join(t.mediaRoot, "transcodes", itemID, profileName)
			_ = os.MkdirAll(outputDir, 0750)

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
