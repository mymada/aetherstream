package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/devuser/aetherstream/pkg/auth"
	"github.com/labstack/echo/v4"
)

// MobileServer wraps mobile-optimized API endpoints.
type MobileServer struct {
	srv *Server
}

// NewMobileServer creates a mobile API server attached to the main API server.
func NewMobileServer(srv *Server) *MobileServer {
	return &MobileServer{srv: srv}
}

// RegisterRoutes sets up mobile-optimized routes under /api/mobile.
func (m *MobileServer) RegisterRoutes(e *echo.Echo) {
	mobile := e.Group("/api/mobile")
	mobile.Use(m.srv.auth.Middleware())
	mobile.Use(SessionTimeout(30*time.Minute, m.srv.db))

	mobile.GET("/dashboard", m.handleMobileDashboard)
	mobile.GET("/continue-watching", m.handleContinueWatching)
	mobile.GET("/libraries/:id/items", m.handleMobileLibraryItems)
	mobile.GET("/items/:id/stream-info", m.handleStreamInfo)
	mobile.POST("/sync/progress", m.handleSyncProgress)
	mobile.GET("/sync/progress", m.handleGetSyncProgress)
	mobile.GET("/notifications", m.handleNotifications)
}

// MobileDashboardItem is a lightweight item representation for mobile clients.
type MobileDashboardItem struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	MediaType    string  `json:"mediaType"`
	DurationSec  float64 `json:"durationSec"`
	ProgressPct  float64 `json:"progressPct"`
	ThumbnailURL string  `json:"thumbnailUrl"`
	IsNew        bool    `json:"isNew"`
}

func (m *MobileServer) handleMobileDashboard(c echo.Context) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	// Return lightweight recent items + continue watching
	libs, err := m.srv.db.ListLibraries()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")
	}

	var recent []MobileDashboardItem
	for _, lib := range libs {
		items, err := m.srv.db.ListItemsByLibrary(lib.ID)
		if err != nil {
			continue
		}
		for _, it := range items {
			recent = append(recent, MobileDashboardItem{
				ID:           it.ID,
				Name:         it.Name,
				MediaType:    it.MediaType,
				DurationSec:  it.DurationSeconds,
				ThumbnailURL: "/api/items/" + it.ID + "/thumbnails/poster",
			})
			if len(recent) >= 20 {
				break
			}
		}
		if len(recent) >= 20 {
			break
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"recent": recent,
	})
}

func (m *MobileServer) handleContinueWatching(c echo.Context) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	progress, err := m.srv.db.GetPlaybackReporting(user.UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")
	}

	return c.JSON(http.StatusOK, progress)
}

func (m *MobileServer) handleMobileLibraryItems(c echo.Context) error {
	libID := c.Param("id")
	limit := 50
	if l := c.QueryParam("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}

	items, err := m.srv.db.ListItemsByLibrary(libID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")
	}

	var result []MobileDashboardItem
	for i, it := range items {
		if i >= limit {
			break
		}
		result = append(result, MobileDashboardItem{
			ID:           it.ID,
			Name:         it.Name,
			MediaType:    it.MediaType,
			DurationSec:  it.DurationSeconds,
			ThumbnailURL: "/api/items/" + it.ID + "/thumbnails/poster",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"libraryId": libID,
		"items":     result,
		"count":     len(result),
	})
}

// StreamInfo provides mobile-optimized stream metadata (codecs, bandwidth).
type StreamInfo struct {
	ItemID            string   `json:"itemId"`
	Name              string   `json:"name"`
	DirectPlay        bool     `json:"directPlay"`
	VideoCodec        string   `json:"videoCodec"`
	AudioCodec        string   `json:"audioCodec"`
	Width             int      `json:"width"`
	Height            int      `json:"height"`
	DurationSec       float64  `json:"durationSec"`
	AvailableProfiles []string `json:"availableProfiles"`
}

func (m *MobileServer) handleStreamInfo(c echo.Context) error {
	itemID := c.Param("id")
	item, err := m.srv.db.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}

	info := StreamInfo{
		ItemID:            item.ID,
		Name:              item.Name,
		VideoCodec:        item.VideoCodec,
		AudioCodec:        item.AudioCodec,
		Width:             item.Width,
		Height:            item.Height,
		DurationSec:       item.DurationSeconds,
		AvailableProfiles: []string{"auto", "720p", "480p", "360p"},
	}

	// Simple direct-play heuristic: H.264 + AAC is widely supported
	info.DirectPlay = (item.VideoCodec == "h264" || item.VideoCodec == "avc") &&
		(item.AudioCodec == "aac" || item.AudioCodec == "mp3")

	return c.JSON(http.StatusOK, info)
}

// SyncProgressRequest is sent by mobile clients to sync playback state.
type SyncProgressRequest struct {
	ItemID          string  `json:"itemId"`
	PositionSeconds float64 `json:"positionSeconds"`
	DurationSeconds float64 `json:"durationSeconds"`
	PercentComplete float64 `json:"percentComplete"`
}

func (m *MobileServer) handleSyncProgress(c echo.Context) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	var req SyncProgressRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	if err := m.srv.db.SavePlaybackProgress(user.UserID, req.ItemID, req.PositionSeconds, req.DurationSeconds, req.PercentComplete); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")
	}
	return c.NoContent(http.StatusNoContent)
}

func (m *MobileServer) handleGetSyncProgress(c echo.Context) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	itemID := c.QueryParam("itemId")
	if itemID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "itemId required")
	}
	progress, err := m.srv.db.GetPlaybackProgress(user.UserID, itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "progress not found")
	}
	return c.JSON(http.StatusOK, progress)
}

// Notification represents a server-side notification for mobile clients.
type Notification struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"createdAt"`
}

func (m *MobileServer) handleNotifications(c echo.Context) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	_ = user
	// Placeholder: in production this would query a notifications table.
	return c.JSON(http.StatusOK, []Notification{
		{
			ID:        "1",
			Title:     "Welcome",
			Message:   "AetherStream mobile API is ready.",
			Type:      "info",
			CreatedAt: time.Now().UTC(),
		},
	})
}
