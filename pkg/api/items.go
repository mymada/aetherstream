package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/devuser/aetherstream/pkg/auth"
	"github.com/devuser/aetherstream/pkg/encoder"
	"github.com/devuser/aetherstream/pkg/probe"
	"github.com/devuser/aetherstream/pkg/thumbnail"
	"github.com/devuser/aetherstream/pkg/ws"
	"github.com/google/uuid"
)

func (s *Server) handleSystemInfo(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"name":    "AetherStream",
		"version": "1.3.0",
		"status":  "ok",
	})
}

func (s *Server) handleSystemHardware(c echo.Context) error {
	caps := encoder.DetectHardwareCapabilities()
	return c.JSON(http.StatusOK, caps)
}

func (s *Server) handleListItems(c echo.Context) error {
	libID := c.QueryParam("library_id")
	if libID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "library_id required")
	}
	items, err := s.db.ListItemsByLibrary(libID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")
	}
	return c.JSON(http.StatusOK, items)
}

func (s *Server) handleGetItem(c echo.Context) error {
	id := c.Param("id")
	item, err := s.db.GetItemByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}
	return c.JSON(http.StatusOK, item)
}

func (s *Server) handleListLibraries(c echo.Context) error {
	libs, err := s.db.ListLibraries()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")
	}
	return c.JSON(http.StatusOK, libs)
}

func (s *Server) handleCreateLibrary(c echo.Context) error {
	var req struct {
		Name      string `json:"name"`
		Path      string `json:"path"`
		MediaType string `json:"media_type"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	if req.Name == "" || req.Path == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name and path required")
	}
	id, err := s.library.CreateLibrary(req.Name, req.Path, req.MediaType)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")
	}
	return c.JSON(http.StatusCreated, map[string]string{
		"id":   id,
		"name": req.Name,
		"path": req.Path,
	})
}

func (s *Server) handleScanLibrary(c echo.Context) error {
	id := c.Param("id")
	if err := s.library.ScanLibrary(id); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "scanning", "library_id": id})
}

func (s *Server) handleListSubtitles(c echo.Context) error {
	itemID := c.Param("id")
	item, err := s.db.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}
	subs, err := probe.ExtractSubtitleTracks(item.Path)
	if err != nil {
		return c.JSON(http.StatusOK, []map[string]interface{}{})
	}
	return c.JSON(http.StatusOK, subs)
}

func (s *Server) handleGetSubtitle(c echo.Context) error {
	itemID := c.Param("id")
	lang := c.Param("lang")

	// Validate itemID (UUID or alphanumeric, no path traversal)
	if !isValidItemID(itemID) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid item id")
	}

	// Validate lang with strict whitelist: ISO 639-1/2 codes optionally with region
	if !isValidLanguageCode(lang) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid language parameter")
	}

	item, err := s.db.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}

	subPath, err := probe.ExtractSubtitleToFile(item.Path, lang)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "subtitle not found")
	}

	// Ensure subtitle path is within allowed directories (temp or thumbnails)
	if !isPathWithinAllowedDirs(subPath, []string{os.TempDir(), "./thumbnails"}) {
		return echo.NewHTTPError(http.StatusInternalServerError, "invalid subtitle path")
	}

	return c.File(subPath)
}

func (s *Server) handleGetThumbnail(c echo.Context) error {
	itemID := c.Param("id")
	thumbType := c.Param("type")
	item, err := s.db.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}
	if item.Path == "" {
		return echo.NewHTTPError(http.StatusNotFound, "no file path")
	}
	var t thumbnail.ThumbnailType
	switch thumbType {
	case "poster":
		t = thumbnail.TypePoster
	case "backdrop":
		t = thumbnail.TypeBackdrop
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "invalid thumbnail type")
	}
	if !s.thumbSvc.Exists(itemID, t) {
		_, _, err = s.thumbSvc.GenerateThumbnails(item.Path, itemID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "thumbnail generation failed")
		}
	}
	thumbPath := s.thumbSvc.Path(itemID, t)
	return c.File(thumbPath)
}

func (s *Server) handleSearch(c echo.Context) error {
	q := c.QueryParam("q")
	if q == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "query parameter 'q' required")
	}
	mediaType := c.QueryParam("type")
	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if _, err := fmt.Sscanf(l, "%d", &limit); err != nil {
			limit = 20
		}
	}
	const maxLimit = 100
	if limit > maxLimit {
		limit = maxLimit
	}
	results, err := s.searcher.SearchItems(q, mediaType, limit)
	if err != nil {
		return echo.NewHTTPError(500, "search failed")
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"query":   q,
		"type":    mediaType,
		"limit":   limit,
		"results": results,
	})
}

func (s *Server) handleListActivity(c echo.Context) error {
	acts, err := s.db.ListActivity(50)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")
	}
	return c.JSON(http.StatusOK, acts)
}

func (s *Server) handleWebSocket(c echo.Context) error {
	return ws.HandleWebSocket(c, s.db)
}

func (s *Server) handlePlaybackWebSocket(c echo.Context) error {
	return ws.HandlePlaybackWebSocketHTTP(c, s.auth)
}

func (s *Server) handleListCollections(c echo.Context) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(401, "unauthorized")
	}
	cols, err := s.db.ListCollections(user.UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")
	}
	return c.JSON(http.StatusOK, cols)
}

func (s *Server) handleCreateCollection(c echo.Context) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(401, "unauthorized")
	}
	var req struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	if req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name required")
	}
	if req.Type == "" {
		req.Type = "collection"
	}
	id := uuid.New().String()
	if err := s.db.CreateCollection(id, user.UserID, req.Name, req.Type); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")
	}
	return c.JSON(http.StatusCreated, map[string]string{"id": id, "name": req.Name, "type": req.Type})
}

func (s *Server) handleGetCollection(c echo.Context) error {
	id := c.Param("id")
	col, items, err := s.db.GetCollectionWithItems(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "collection not found")
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"collection": col,
		"items":      items,
	})
}

func (s *Server) handleAddToCollection(c echo.Context) error {
	colID := c.Param("id")
	var req struct {
		ItemID string `json:"item_id"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	if err := s.db.AddItemToCollection(colID, req.ItemID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Server) handleRemoveFromCollection(c echo.Context) error {
	colID := c.Param("id")
	itemID := c.Param("item_id")
	if err := s.db.RemoveItemFromCollection(colID, itemID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Server) handleSaveProgress(c echo.Context) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	itemID := c.Param("id")
	var req struct {
		PositionSeconds float64 `json:"position_seconds"`
		DurationSeconds float64 `json:"duration_seconds"`
		PercentComplete float64 `json:"percent_complete"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	if err := s.db.SavePlaybackProgress(user.UserID, itemID, req.PositionSeconds, req.DurationSeconds, req.PercentComplete); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Server) handleGetProgress(c echo.Context) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	itemID := c.Param("id")
	progress, err := s.db.GetPlaybackProgress(user.UserID, itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "playback progress not found")
	}
	return c.JSON(http.StatusOK, progress)
}

func (s *Server) handleMarkWatched(c echo.Context) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	itemID := c.Param("id")
	var req struct {
		PositionSeconds float64 `json:"position_seconds"`
		DurationSeconds float64 `json:"duration_seconds"`
		Watched         bool    `json:"watched"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	if err := s.db.SaveWatchHistory(user.UserID, itemID, req.PositionSeconds, req.DurationSeconds, req.Watched); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")
	}
	return c.NoContent(http.StatusNoContent)
}

func isValidItemID(id string) bool {
	// Accept UUIDs or URL-safe base64 / alphanumeric IDs
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, id)
	return matched
}

func isValidLanguageCode(lang string) bool {
	// ISO 639-1 (2 letters) or ISO 639-2 (3 letters), optionally with region subtag
	matched, _ := regexp.MatchString(`^[a-zA-Z]{2,3}(-[a-zA-Z]{2})?$`, lang)
	return matched
}

func isPathWithinAllowedDirs(path string, allowedDirs []string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, dir := range allowedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(absDir, absPath)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(rel, "..") && rel != ".." {
			return true
		}
	}
	return false
}

func (s *Server) handlePlaybackReporting(c echo.Context) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	userID := c.Param("id")
	if user.UserID != userID && user.Role != "admin" {
		return echo.NewHTTPError(http.StatusForbidden, "forbidden")
	}
	report, err := s.db.GetPlaybackReporting(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")
	}
	return c.JSON(http.StatusOK, report)
}
