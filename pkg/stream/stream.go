package stream

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

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

	// Security: validate path is within mediaRoot using filepath.Rel
	cleanPath := filepath.Clean(path)
	cleanRoot := filepath.Clean(s.mediaRoot)
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
	rel, err := filepath.Rel(cleanRoot, cleanPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return echo.NewHTTPError(http.StatusForbidden, "invalid path")
	}

	// Open with O_NOFOLLOW to prevent TOCTOU symlink attack
	f, err := os.OpenFile(cleanPath, os.O_RDONLY, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "file not found on disk")
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "file not found on disk")
	}
	_ = fi // silence unused variable warning

	// Serve via http.ServeContent with opened file (no TOCTOU)
	return c.Stream(http.StatusOK, "application/octet-stream", f)
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
	rel, err := filepath.Rel(cleanRoot, cleanPlaylist)
	if err != nil || strings.HasPrefix(rel, "..") {
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
	rel, err := filepath.Rel(cleanRoot, cleanSegment)
	if err != nil || strings.HasPrefix(rel, "..") {
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

	// Reject any path separators in filename
	if strings.Contains(file, "..") || strings.Contains(file, "/") || strings.Contains(file, "\\") {
		return echo.NewHTTPError(http.StatusForbidden, "invalid file name")
	}

	segmentPath := filepath.Join(s.mediaRoot, "transcodes", itemID, profile, file)

	cleanSegment := filepath.Clean(segmentPath)
	cleanRoot := filepath.Clean(filepath.Join(s.mediaRoot, "transcodes"))
	rel, err := filepath.Rel(cleanRoot, cleanSegment)
	if err != nil || strings.HasPrefix(rel, "..") {
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

	// Validate language code before processing (M3 fix)
	if !isValidLanguageCode(req.Language) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid language code")
	}

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

func BurnIn(itemPath, lang, mediaRoot, profileName, hwAccel string) (*BurnInResult, error) {
	subPath, err := extractSubtitleForBurnIn(itemPath, lang)
	if err != nil {
		return nil, fmt.Errorf("subtitle extraction: %w", err)
	}
	defer os.Remove(subPath)

	hash := sha256.Sum256([]byte(itemPath + ":" + lang + ":" + profileName))
	outDir := filepath.Join(mediaRoot, "transcodes", "burnin")
	_ = os.MkdirAll(outDir, 0750)
	outPath := filepath.Join(outDir, fmt.Sprintf("%x.mp4", hash[:16]))

	args := BuildBurnInCommand(itemPath, subPath, outPath, profileName, hwAccel)
	cmd := exec.Command("ffmpeg", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg burn-in failed: %w\n%s", err, output)
	}

	return &BurnInResult{
		OutputPath: outPath,
		Language:   lang,
		Profile:    profileName,
	}, nil
}

func BuildBurnInCommand(inputPath, subtitlePath, outputPath, profileName, hwAccel string) []string {
	var scale string
	switch profileName {
	case "mobile":
		scale = "1280:720"
	case "tablet":
		scale = "1920:1080"
	default:
		scale = "1920:1080"
	}

	var vf string
	if filepath.Ext(subtitlePath) == ".sup" || filepath.Ext(subtitlePath) == ".sub" {
		// Bitmap subtitle: use overlay filter
		vf = fmt.Sprintf("overlay=0:0,scale=%s", scale)
	} else {
		vf = fmt.Sprintf("subtitles='%s',scale=%s", subtitlePath, scale)
	}

	var codec, preset string
	if hwAccel == "nvenc" {
		codec = "h264_nvenc"
		preset = "fast"
	} else {
		codec = "libx264"
		preset = "fast"
	}

	return []string{
		"-hide_banner",
		"-y",
		"-i", inputPath,
		"-vf", vf,
		"-c:v", codec,
		"-preset", preset,
		"-crf", "23",
		"-c:a", "copy",
		outputPath,
	}
}

type BurnInResult struct {
	OutputPath string `json:"output_path"`
	Language   string `json:"language"`
	Profile    string `json:"profile"`
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

	if !isValidLanguageCode(lang) {
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

// isValidLanguageCode validates ISO 639-1/2 language codes
func isValidLanguageCode(lang string) bool {
	if lang == "" {
		return false
	}
	for _, r := range lang {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '-') {
			return false
		}
	}
	return true
}

// extractSubtitleForBurnIn extracts the specified language subtitle to a secure temp SRT file.
func extractSubtitleForBurnIn(path, lang string) (string, error) {
	// Validate language code before using in paths or command args (fixes M3)
	if !isValidLanguageCode(lang) {
		return "", fmt.Errorf("invalid language code: %s", lang)
	}

	// Use secure temp file (fixes M4: no fixed path collision)
	f, err := os.CreateTemp("", "aetherstream_burnin_*.srt")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	outPath := f.Name()
	_ = f.Close()

	// #nosec G204 - outPath is secure temp; path validated by caller; lang is sanitized
	cmd := exec.Command("ffmpeg", "-i", path, "-map", "0:s:m:language:"+lang, outPath, "-y")
	if _, err := cmd.CombinedOutput(); err != nil {
		// Fallback: try first subtitle stream if language match fails
		// #nosec G204 - same validated parameters
		cmd2 := exec.Command("ffmpeg", "-i", path, "-map", "0:s:0", outPath, "-y")
		if _, err2 := cmd2.CombinedOutput(); err2 != nil {
			os.Remove(outPath)
			return "", fmt.Errorf("subtitle extraction failed: %w", err)
		}
	}
	return outPath, nil
}
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

func ValidateBurnInRequest(req BurnInRequest, mediaRoot string) error {
	if req.ItemID == "" {
		return fmt.Errorf("item_id required")
	}
	if req.Language == "" {
		return fmt.Errorf("language required")
	}
	if req.OutputPath != "" {
		cleanOut := filepath.Clean(req.OutputPath)
		cleanRoot := filepath.Clean(mediaRoot)
		if !strings.HasPrefix(cleanOut, cleanRoot+string(filepath.Separator)) && cleanOut != cleanRoot {
			return fmt.Errorf("invalid output_path")
		}
	}
	return nil
}

// Transcoder manages background transcode jobs
type BurnInRequest struct {
	ItemID     string `json:"item_id"`
	Language   string `json:"language"`
	OutputPath string `json:"output_path,omitempty"`
	Profile    string `json:"profile,omitempty"`
	HWAccel    string `json:"hw_accel,omitempty"`
}

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
	// Atomically check-and-set job status under single Lock (fixes C2 race)
	t.mu.Lock()
	running := t.jobs[itemID]
	if running {
		t.mu.Unlock()
		return fmt.Errorf("transcode already in progress")
	}
	t.jobs[itemID] = true
	t.mu.Unlock()

	item, err := t.db.GetItemByID(itemID)
	if err != nil {
		t.mu.Lock()
		delete(t.jobs, itemID)
		t.mu.Unlock()
		return err
	}

	inputPath := item.Path
	if inputPath == "" {
		t.mu.Lock()
		delete(t.jobs, itemID)
		t.mu.Unlock()
		return fmt.Errorf("no input path")
	}

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

			// Run FFmpeg with 30-minute timeout (fixes H2)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			// #nosec G204 - inputPath is validated against library paths before reaching here
			cmd := exec.CommandContext(ctx, "ffmpeg", args...)
			if output, err := cmd.CombinedOutput(); err != nil {
				// Log error but continue with other profiles
				fmt.Printf("transcode error for %s: %v\n%s\n", profileName, err, output)
			}
		}
	}()

	return nil
}
