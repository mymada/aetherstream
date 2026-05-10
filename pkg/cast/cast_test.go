package cast

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChromecastController(t *testing.T) {
	ctrl := NewChromecastController("http://localhost:8080")
	assert.NotNil(t, ctrl)
}

func TestAirPlayController(t *testing.T) {
	ctrl := NewAirPlayController("http://localhost:8080")
	assert.NotNil(t, ctrl)
}

func TestCastDevice(t *testing.T) {
	dev := &CastDevice{
		ID:      "dev-1",
		Name:    "Test TV",
		Type:    "chromecast",
		Address: "192.168.1.100",
		Port:    8009,
	}
	assert.Equal(t, "Test TV", dev.Name)
	assert.Equal(t, "chromecast", dev.Type)
}

func TestSession(t *testing.T) {
	session := &Session{
		ID:       "sess-1",
		DeviceID: "dev-1",
		MediaURL: "http://localhost/videos/1.m3u8",
		ItemID:   "item-1",
		State:    "playing",
	}
	assert.Equal(t, "sess-1", session.ID)
	assert.Equal(t, "playing", session.State)
}
