package m3u

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPackageCompiles(t *testing.T) {
	assert.True(t, true, "m3u package should compile")
}

func TestExportM3U(t *testing.T) {
	entries := []PlaylistEntry{
		{Title: "Video 1", Duration: 120, URL: "http://example.com/1.m3u8"},
		{Title: "Video 2", Duration: 300, URL: "http://example.com/2.m3u8", Group: "Movies"},
	}
	out := ExportM3U(entries)
	assert.True(t, strings.HasPrefix(out, "#EXTM3U"))
	assert.Contains(t, out, "#EXTINF:120,Video 1")
	assert.Contains(t, out, "#EXTINF:300 group-title=\"Movies\",Video 2")
	assert.Contains(t, out, "http://example.com/1.m3u8")
}

func TestExportM3UWithMetadata(t *testing.T) {
	entries := []PlaylistEntry{
		{Title: "Live", Duration: -1, URL: "http://example.com/live.m3u8"},
	}
	out := ExportM3UWithMetadata(entries, "My Playlist")
	assert.Contains(t, out, "#PLAYLIST:My Playlist")
	assert.Contains(t, out, "#EXTINF:-1,Live")
}
