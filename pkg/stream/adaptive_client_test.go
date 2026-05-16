package stream

import (
	"context"
	"testing"

	"github.com/devuser/aetherstream/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupAdaptiveDB creates an in-memory DB with a library and test items.
func setupAdaptiveDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.New(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate())
	t.Cleanup(func() { database.Close() })
	require.NoError(t, database.CreateLibrary("lib-1", "Test", "/tmp/lib", "movies"))
	return database
}

// seedAdaptiveItem inserts a video item with given dimensions.
func seedAdaptiveItem(t *testing.T, database *db.DB, id string, width, height int) {
	t.Helper()
	err := database.CreateItem(id, "lib-1", "/media/test.mp4", "Test Item", "video", "mp4", 1000000, 120.0, width, height, "h264", "aac")
	require.NoError(t, err)
}

// --- AdaptiveProfileSelector.SelectProfile ---

func TestSelectProfile_Mobile(t *testing.T) {
	database := setupAdaptiveDB(t)
	seedAdaptiveItem(t, database, "item-1", 1920, 1080)
	sel := NewAdaptiveSelector(database)

	p, err := sel.SelectProfile("item-1", 1000, "mobile")
	require.NoError(t, err)
	assert.Equal(t, "mobile_low", p.Name)

	p, err = sel.SelectProfile("item-1", 2000, "mobile")
	require.NoError(t, err)
	assert.Equal(t, "mobile", p.Name)
}

func TestSelectProfile_Tablet(t *testing.T) {
	database := setupAdaptiveDB(t)
	seedAdaptiveItem(t, database, "item-1", 1920, 1080)
	sel := NewAdaptiveSelector(database)

	p, err := sel.SelectProfile("item-1", 2000, "tablet")
	require.NoError(t, err)
	assert.Equal(t, "mobile", p.Name)

	p, err = sel.SelectProfile("item-1", 5000, "tablet")
	require.NoError(t, err)
	assert.Equal(t, "tablet", p.Name)
}

func TestSelectProfile_TV(t *testing.T) {
	database := setupAdaptiveDB(t)
	seedAdaptiveItem(t, database, "item-hd", 1920, 1080)
	seedAdaptiveItem(t, database, "item-4k", 3840, 2160)
	sel := NewAdaptiveSelector(database)

	p, err := sel.SelectProfile("item-hd", 5000, "tv")
	require.NoError(t, err)
	assert.Equal(t, "tablet", p.Name)

	p, err = sel.SelectProfile("item-hd", 10000, "tv")
	require.NoError(t, err)
	assert.Equal(t, "tv", p.Name)

	p, err = sel.SelectProfile("item-4k", 10000, "tv")
	require.NoError(t, err)
	assert.Equal(t, "tv_4k", p.Name)
}

func TestSelectProfile_Auto(t *testing.T) {
	database := setupAdaptiveDB(t)
	seedAdaptiveItem(t, database, "item-1", 1920, 1080)
	sel := NewAdaptiveSelector(database)

	cases := []struct {
		bw   int
		want string
	}{
		{500, "mobile_low"},
		{2000, "mobile"},
		{4000, "tablet"},
		{10000, "tv"},
		{20000, "tv_4k"},
	}
	for _, tc := range cases {
		p, err := sel.SelectProfile("item-1", tc.bw, "auto")
		require.NoError(t, err)
		assert.Equal(t, tc.want, p.Name, "bw=%d", tc.bw)
	}
}

func TestSelectProfile_DefaultBandwidth(t *testing.T) {
	database := setupAdaptiveDB(t)
	seedAdaptiveItem(t, database, "item-1", 1920, 1080)
	sel := NewAdaptiveSelector(database)

	// bandwidth=0 defaults to 5000 → tablet in auto mode
	p, err := sel.SelectProfile("item-1", 0, "auto")
	require.NoError(t, err)
	assert.Equal(t, "tablet", p.Name)
}

func TestSelectProfile_NotFound(t *testing.T) {
	database := setupAdaptiveDB(t)
	sel := NewAdaptiveSelector(database)

	_, err := sel.SelectProfile("nonexistent", 5000, "auto")
	assert.Error(t, err)
}

// --- ParseUserAgent — missing branches ---

func TestParseUserAgent_AppleTV(t *testing.T) {
	caps := ParseUserAgent("AppleTV6,2/11.1")
	require.NotNil(t, caps)
	assert.Contains(t, caps.VideoCodecs, "hevc")
	assert.Contains(t, caps.VideoCodecs, "av1")
	assert.Equal(t, 40000, caps.MaxBitrate)

	caps2 := ParseUserAgent("appletv/1.0 (iOS 13.0)")
	require.NotNil(t, caps2)
	assert.Contains(t, caps2.VideoCodecs, "hevc")
}

func TestParseUserAgent_Roku(t *testing.T) {
	caps := ParseUserAgent("Roku/DVP-10.5 (10.5.0.4067)")
	require.NotNil(t, caps)
	assert.Contains(t, caps.VideoCodecs, "h264")
	assert.Contains(t, caps.VideoCodecs, "hevc")
	assert.Equal(t, 20000, caps.MaxBitrate)
}

func TestParseUserAgent_SmartTV_Tizen(t *testing.T) {
	caps := ParseUserAgent("Mozilla/5.0 (SMART-TV; Linux; Tizen 5.0) AppleWebKit/538.1")
	require.NotNil(t, caps)
	assert.Contains(t, caps.VideoCodecs, "h264")
	assert.Equal(t, 15000, caps.MaxBitrate)
}

func TestParseUserAgent_SmartTV_WebOS(t *testing.T) {
	caps := ParseUserAgent("Mozilla/5.0 (Web0S; Linux/SmartTV) AppleWebKit/537.36 webos/4.0")
	require.NotNil(t, caps)
	assert.Contains(t, caps.VideoCodecs, "hevc")
	assert.Equal(t, 15000, caps.MaxBitrate)
}

func TestParseUserAgent_SmartTV_Generic(t *testing.T) {
	caps := ParseUserAgent("Mozilla/5.0 (smart-tv; Linux) SmartTV/1.0")
	require.NotNil(t, caps)
	assert.Equal(t, 15000, caps.MaxBitrate)
}

// --- CanDirectPlay — zero bitrate / unlimited ---

func TestCanDirectPlay_ZeroBitrate(t *testing.T) {
	caps := &ClientCapabilities{
		VideoCodecs: []string{"h264"},
		AudioCodecs: []string{"aac"},
		Containers:  []string{"mp4"},
		MaxBitrate:  0,
	}
	assert.True(t, caps.CanDirectPlay("h264", "aac", "mp4", 999999))
}

// --- rewritePlaylistSegments ---

func TestRewritePlaylistSegments_Basic(t *testing.T) {
	playlist := "#EXTM3U\n#EXT-X-VERSION:3\n#EXTINF:4.0,\nseg000.ts\nseg001.ts\n"
	out := rewritePlaylistSegments(playlist, "mytoken", 0, 0)
	assert.Contains(t, out, "seg000.ts?token=mytoken&audio=0&start=0")
	assert.Contains(t, out, "seg001.ts?token=mytoken&audio=0&start=0")
	assert.Contains(t, out, "#EXTM3U")
}

func TestRewritePlaylistSegments_WithAudioAndStart(t *testing.T) {
	playlist := "#EXTINF:4.0,\nseg000.ts\n"
	out := rewritePlaylistSegments(playlist, "tok", 2, 120)
	assert.Contains(t, out, "seg000.ts?token=tok&audio=2&start=120")
}

func TestRewritePlaylistSegments_NonTSLines(t *testing.T) {
	playlist := "#EXTINF:4.0,\nsome.m3u8\nseg.ts\n"
	out := rewritePlaylistSegments(playlist, "tok", 0, 0)
	assert.Contains(t, out, "some.m3u8")
	// .m3u8 lines are NOT rewritten (no .ts suffix)
	assert.NotContains(t, out, "some.m3u8?token")
	assert.Contains(t, out, "seg.ts?token=tok")
}

func TestRewritePlaylistSegments_Empty(t *testing.T) {
	out := rewritePlaylistSegments("", "tok", 0, 0)
	assert.Equal(t, "", out)
}

// --- CancelAllSessions ---

func TestCancelAllSessions_Empty(t *testing.T) {
	database := setupAdaptiveDB(t)
	srv := NewServer(database, "/tmp/media")
	// Should not panic with empty sessions map
	srv.CancelAllSessions()
}

func TestCancelAllSessions_WithSessions(t *testing.T) {
	database := setupAdaptiveDB(t)
	srv := NewServer(database, "/tmp/media")

	// Inject a cancellable session
	_, cancel := context.WithCancel(context.Background())
	srv.mu.Lock()
	srv.sessions["sess-1"] = &TranscodeSession{
		ID:     "sess-1",
		ItemID: "item-1",
		Cancel: cancel,
	}
	srv.mu.Unlock()

	srv.CancelAllSessions()

	srv.mu.Lock()
	remaining := len(srv.sessions)
	srv.mu.Unlock()
	assert.Equal(t, 0, remaining)
}
