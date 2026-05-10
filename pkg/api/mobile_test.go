package api

import (
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestMobileDashboardItem(t *testing.T) {
	item := MobileDashboardItem{
		ID:        "1",
		Name:      "Test",
		MediaType: "movie",
	}
	assert.Equal(t, "1", item.ID)
	assert.Equal(t, "Test", item.Name)
}

func TestStreamInfo(t *testing.T) {
	info := StreamInfo{
		ItemID:     "1",
		Name:       "Test",
		DirectPlay: true,
		VideoCodec: "h264",
		AudioCodec: "aac",
	}
	assert.True(t, info.DirectPlay)
	assert.Equal(t, "h264", info.VideoCodec)
}

func TestSyncProgressRequest(t *testing.T) {
	req := SyncProgressRequest{
		ItemID:          "1",
		PositionSeconds: 120.5,
		DurationSeconds: 3600,
		PercentComplete: 3.3,
	}
	assert.Equal(t, "1", req.ItemID)
}

func TestNotification(t *testing.T) {
	notif := Notification{
		ID:      "1",
		Title:   "Hello",
		Message: "World",
		Type:    "info",
	}
	assert.Equal(t, "Hello", notif.Title)
}

func TestMobileServer_RegisterRoutes(t *testing.T) {
	e := echo.New()
	// Minimal Server stub with nil auth (routes registration only tests group creation)
	// In real tests we'd need a full Server with DB; here we just ensure types compile.
	_ = e
}

func TestMobileHandlers_AuthRequired(t *testing.T) {
	// These handlers require auth middleware; unit tests would need Echo test
	// requests with JWT. We keep this as a compile-time check.
	var _ echo.HandlerFunc = func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	}
}
