package stream

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/devuser/aetherstream/pkg/db"
	"github.com/devuser/aetherstream/pkg/encoder"
	"github.com/labstack/echo/v4"
)

// AdaptiveProfileSelector chooses best profile based on bandwidth and device
type AdaptiveProfileSelector struct {
	db *db.DB
}

// NewAdaptiveSelector creates bandwidth-aware profile selector
func NewAdaptiveSelector(database *db.DB) *AdaptiveProfileSelector {
	return &AdaptiveProfileSelector{db: database}
}

// SelectProfile returns optimal profile based on client info
func (s *AdaptiveProfileSelector) SelectProfile(itemID string, bandwidthKbps int, deviceType string) (encoder.Profile, error) {
	// Default bandwidth if unknown
	if bandwidthKbps <= 0 {
		bandwidthKbps = 5000 // assume 5 Mbps
	}

	// Get item info for source resolution
	item, err := s.db.GetItemByID(itemID)
	if err != nil {
		return encoder.Profile{}, err
	}

	srcWidth := item.Width
	srcHeight := item.Height

	// Device-specific overrides
	switch deviceType {
	case "mobile":
		if bandwidthKbps < 1500 {
			return encoder.GetProfileByName("mobile_low"), nil
		}
		return encoder.GetProfileByName("mobile"), nil
	case "tablet":
		if bandwidthKbps < 3000 {
			return encoder.GetProfileByName("mobile"), nil
		}
		return encoder.GetProfileByName("tablet"), nil
	case "tv":
		if bandwidthKbps < 8000 {
			return encoder.GetProfileByName("tablet"), nil
		}
		if srcWidth >= 3840 || srcHeight >= 2160 {
			return encoder.GetProfileByName("tv_4k"), nil
		}
		return encoder.GetProfileByName("tv"), nil
	default:
		// Auto-select based on bandwidth
		switch {
		case bandwidthKbps < 1500:
			return encoder.GetProfileByName("mobile_low"), nil
		case bandwidthKbps < 3000:
			return encoder.GetProfileByName("mobile"), nil
		case bandwidthKbps < 6000:
			return encoder.GetProfileByName("tablet"), nil
		case bandwidthKbps < 15000:
			return encoder.GetProfileByName("tv"), nil
		default:
			return encoder.GetProfileByName("tv_4k"), nil
		}
	}
}

// RegisterAdaptiveRoutes adds bandwidth-aware streaming endpoints (protected by auth middleware)
func RegisterAdaptiveRoutes(e *echo.Echo, database *db.DB, mediaRoot string, authMiddleware echo.MiddlewareFunc) {
	selector := NewAdaptiveSelector(database)

	g := e.Group("/videos")
	g.Use(authMiddleware)
	g.GET("/:id/adaptive.m3u8", func(c echo.Context) error {
		itemID := c.Param("id")
		bandwidthStr := c.QueryParam("bandwidth")
		deviceType := c.QueryParam("device")

		bandwidthKbps, _ := strconv.Atoi(bandwidthStr)
		profile, err := selector.SelectProfile(itemID, bandwidthKbps, deviceType)
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound, "item not found")
		}

		// Return a single-variant playlist for the selected profile
		playlist := fmt.Sprintf("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d\n%s/playlist.m3u8\n",
			profile.VideoBitrate*1000, profile.Width, profile.Height, profile.Name)

		c.Response().Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		return c.String(http.StatusOK, playlist)
	})

	// Hardware acceleration auto-detect endpoint
	g.GET("/system/hwaccel", func(c echo.Context) error {
		hw := encoder.DetectHWAccel()
		return c.JSON(http.StatusOK, map[string]string{
			"hwaccel": hw,
			"status":  "ok",
		})
	})
}
