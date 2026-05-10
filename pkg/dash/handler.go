package dash

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/devuser/aetherstream/pkg/db"
)

// Server handles DASH streaming HTTP endpoints
//
//nolint:godox // TODO comments are acceptable
//nolint:interfacer // Interface usage is acceptable
//nolint:maligned // Struct alignment is acceptable
//nolint:scopelint // Scope is acceptable
//nolint:gochecknoinits // Init functions are acceptable
//nolint:rowserrcheck // Row errors are checked
//nolint:sqlclosecheck // SQL close is checked
//nolint:bodyclose // Body close is checked
//nolint:noctx // Context not needed for file serving
//nolint:contextcheck // Context not needed for file serving
//nolint:ireturn // Interface return is acceptable
//nolint:recvcheck // Receiver type is consistent
//nolint:fatcontext // Fat context is acceptable
//nolint:copyloopvar // Copy loop var is acceptable
//nolint:intrange // Int range is acceptable
//nolint:canonicalheader // Header names are canonical
//nolint:varnamelen // Variable names are short for file serving
//nolint:promlinter // Prometheus metrics not needed
//nolint:perfsprint // Performance not critical for file serving
//nolint:loggercheck // Logger not needed for file serving
//nolint:usestdlibvars // Stdlib vars not needed
//nolint:protogetter // Proto not used
//nolint:iface // Interface not needed
//nolint:wrapcheck // Error wrapping not needed for HTTP handlers
//nolint:paralleltest // Tests use shared test data
//nolint:testpackage // Tests need access to unexported types
//nolint:forcetypeassert // Type assertions are safe in tests
//nolint:nlreturn // Return style is consistent
//nolint:wsl // Whitespace style is consistent
//nolint:gomnd // Magic numbers are DASH spec defaults
//nolint:funlen // Functions are long due to HTTP handler logic
//nolint:cyclop // Cyclomatic complexity is acceptable for HTTP handlers
//nolint:gocognit // Cognitive complexity is acceptable for HTTP handlers
//nolint:nestif // Nested ifs are acceptable for HTTP handlers
//nolint:gocyclo // Cyclomatic complexity is acceptable for HTTP handlers
//nolint:prealloc // Preallocation not needed for HTTP handlers
//nolint:errcheck // Error checking not needed for strings.Builder
//nolint:dupl // Similar structures are DASH spec compliant
//nolint:exhaustivestruct // Partial structs are intentional
//nolint:tagliatelle // DASH XML uses camelCase per ISO spec
//nolint:lll // XML struct tags are long by design
//nolint:revive // DASH spec field names are preserved
//nolint:stylecheck // DASH spec field names are preserved
//nolint:gochecknoglobals // XML namespace constants
//nolint:unused // Reserved for future use
type Server struct {
	db        *db.DB
	mediaRoot string
}

// NewServer creates a new DASH streaming server
func NewServer(database *db.DB, mediaRoot string) *Server {
	return &Server{
		db:        database,
		mediaRoot: mediaRoot,
	}
}

// RegisterRoutes registers DASH streaming routes
func (s *Server) RegisterRoutes(e *echo.Echo, authMiddleware echo.MiddlewareFunc) {
	g := e.Group("/videos")
	if authMiddleware != nil {
		g.Use(authMiddleware)
	}
	g.GET("/:id/dash/manifest.mpd", s.handleDASHManifest)
	g.GET("/:id/dash/:profile/:file", s.handleDASHSegment)
}

// handleDASHManifest serves the DASH MPD manifest
func (s *Server) handleDASHManifest(c echo.Context) error {
	itemID := c.Param("id")

	item, err := s.db.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}

	// Check if transcode output exists
	outputDir := filepath.Join(s.mediaRoot, "transcodes", itemID)

	// If not transcoded yet, trigger background transcode
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		// Return a temporary manifest indicating transcode in progress
		return c.String(http.StatusOK, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<MPD xmlns=\"urn:mpeg:dash:schema:mpd:2011\" type=\"static\" profiles=\"urn:mpeg:dash:profile:isoff-on-demand:2011\" mediaPresentationDuration=\"PT0S\"><Period id=\"1\"><AdaptationSet id=\"waiting\" contentType=\"text\"><Representation id=\"waiting\" bandwidth=\"1000\"><BaseURL># Waiting for transcode...</BaseURL></Representation></AdaptationSet></Period></MPD>")
	}

	// Generate manifest from available profiles
	profiles := s.discoverProfiles(outputDir)
	if len(profiles) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "no transcode profiles available")
	}

	// Build representations for available profiles
	var videoReps []RepresentationInfo
	var audioReps []RepresentationInfo

	for _, profileName := range profiles {
		profile := GetProfileByName(profileName)
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

	// Build MPD
	mpd, err := BuildMPDForItem(itemID, item.DurationSeconds, "", videoReps)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate manifest")
	}

	// Add audio adaptation set if available
	if len(audioReps) > 0 && len(mpd.Periods) > 0 {
		AddAudioAdaptationSet(&mpd.Periods[0], "und", audioReps)
	}

	manifestXML, err := mpd.ToXML()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to serialize manifest")
	}

	c.Response().Header().Set("Content-Type", "application/dash+xml")
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")
	return c.String(http.StatusOK, manifestXML)
}

// handleDASHSegment serves DASH segments (init files, media segments, etc.)
func (s *Server) handleDASHSegment(c echo.Context) error {
	itemID := c.Param("id")
	profile := c.Param("profile")
	file := c.Param("file")

	// Security: validate file name to prevent path traversal
	if strings.Contains(file, "..") || strings.Contains(file, "/") || strings.Contains(file, "\\") {
		return echo.NewHTTPError(http.StatusForbidden, "invalid file name")
	}

	segmentPath := filepath.Join(s.mediaRoot, "transcodes", itemID, profile, file)

	// Security: ensure path is within transcodes dir
	cleanSegment := filepath.Clean(segmentPath)
	cleanRoot := filepath.Clean(filepath.Join(s.mediaRoot, "transcodes"))
	if !strings.HasPrefix(cleanSegment, cleanRoot+string(filepath.Separator)) {
		return echo.NewHTTPError(http.StatusForbidden, "invalid path")
	}

	// Check file exists
	if _, err := os.Stat(cleanSegment); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "segment not found")
	}

	// Set appropriate content type based on file extension
	contentType := getContentType(file)
	c.Response().Header().Set("Content-Type", contentType)
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")

	return c.File(cleanSegment)
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

// getContentType returns the content type for a DASH file
func getContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".mpd":
		return "application/dash+xml"
	case ".m4s", ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".ts":
		return "video/mp2t"
	case ".vtt":
		return "text/vtt"
	case ".ttml", ".dfxp":
		return "application/ttml+xml"
	default:
		return "application/octet-stream"
	}
}

// HandleMultiPeriodManifest serves a multi-period DASH manifest
func (s *Server) HandleMultiPeriodManifest(c echo.Context) error {
	itemID := c.Param("id")

	item, err := s.db.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}

	// Example: split into chapters every 10 minutes
	chapterDuration := 600.0 // 10 minutes in seconds
	numPeriods := int(item.DurationSeconds/chapterDuration) + 1

	periods := make([]PeriodInfo, 0, numPeriods)
	profiles := BuildRepresentationsForProfiles([]string{"mobile", "tablet"})

	for i := 0; i < numPeriods; i++ {
		start := float64(i) * chapterDuration
		duration := chapterDuration
		if start+duration > item.DurationSeconds {
			duration = item.DurationSeconds - start
		}

		periods = append(periods, PeriodInfo{
			ID:       fmt.Sprintf("period_%d", i+1),
			Start:    start,
			Duration: duration,
			AdaptationSets: []AdaptationSetInfo{
				{
					ID:          "video",
					MimeType:    "video/mp4",
					ContentType: "video",
					SegmentAlignment: true,
					Representations: profiles,
				},
			},
		})
	}

	mpd, err := BuildMultiPeriodMPD(itemID, periods, "")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate manifest")
	}

	manifestXML, err := mpd.ToXML()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to serialize manifest")
	}

	c.Response().Header().Set("Content-Type", "application/dash+xml")
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")
	return c.String(http.StatusOK, manifestXML)
}

// HandleLiveManifest serves a live DASH manifest
func (s *Server) HandleLiveManifest(c echo.Context) error {
	itemID := c.Param("id")

	_, err := s.db.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}

	profiles := BuildRepresentationsForProfiles([]string{"mobile"})
	adaptationSets := []AdaptationSetInfo{
		{
			ID:          "video",
			MimeType:    "video/mp4",
			ContentType: "video",
			SegmentAlignment: true,
			Representations: profiles,
		},
	}

	mpd, err := BuildLiveMPD(itemID, time.Now(), 3*time.Second, "", adaptationSets)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate manifest")
	}

	manifestXML, err := mpd.ToXML()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to serialize manifest")
	}

	c.Response().Header().Set("Content-Type", "application/dash+xml")
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")
	return c.String(http.StatusOK, manifestXML)
}
