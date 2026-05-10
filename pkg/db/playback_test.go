package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlaybackProgressCRUD(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Migrate())

	// Seed user and item
	require.NoError(t, db.CreateUser("u1", "alice", "hash", "user"))
	require.NoError(t, db.CreateItem("i1", "lib1", "/a.mkv", "a", "video", "mkv", 1000, 120.0, 1920, 1080, "h264", "aac"))

	// Save progress
	err = db.SavePlaybackProgress("u1", "i1", 45.5, 120.0, 37.9)
	require.NoError(t, err)

	// Get progress
	p, err := db.GetPlaybackProgress("u1", "i1")
	require.NoError(t, err)
	assert.Equal(t, "u1", p.UserID)
	assert.Equal(t, "i1", p.ItemID)
	assert.InDelta(t, 45.5, p.PositionSeconds, 0.001)
	assert.InDelta(t, 120.0, p.DurationSeconds, 0.001)
	assert.InDelta(t, 37.9, p.PercentComplete, 0.001)

	// Update progress
	require.NoError(t, db.SavePlaybackProgress("u1", "i1", 90.0, 120.0, 75.0))
	p, err = db.GetPlaybackProgress("u1", "i1")
	require.NoError(t, err)
	assert.InDelta(t, 90.0, p.PositionSeconds, 0.001)

	// Delete progress
	require.NoError(t, db.DeletePlaybackProgress("u1", "i1"))
	_, err = db.GetPlaybackProgress("u1", "i1")
	assert.Error(t, err)
}

func TestWatchHistoryCRUD(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Migrate())

	require.NoError(t, db.CreateUser("u1", "alice", "hash", "user"))
	require.NoError(t, db.CreateItem("i1", "lib1", "/a.mkv", "a", "video", "mkv", 1000, 120.0, 1920, 1080, "h264", "aac"))

	// Save watch history
	require.NoError(t, db.SaveWatchHistory("u1", "i1", 120.0, 120.0, true))

	// Get watch history
	h, err := db.GetWatchHistory("u1", "i1")
	require.NoError(t, err)
	assert.Equal(t, "u1", h.UserID)
	assert.Equal(t, "i1", h.ItemID)
	assert.True(t, h.Watched)
	assert.InDelta(t, 120.0, h.PositionSeconds, 0.001)

	// List watch history
	list, err := db.ListWatchHistoryByUser("u1", 10)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.True(t, list[0].Watched)

	// Update to unwatched
	require.NoError(t, db.SaveWatchHistory("u1", "i1", 30.0, 120.0, false))
	h, err = db.GetWatchHistory("u1", "i1")
	require.NoError(t, err)
	assert.False(t, h.Watched)
	assert.InDelta(t, 30.0, h.PositionSeconds, 0.001)
}

func TestPlaybackReporting(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Migrate())

	require.NoError(t, db.CreateUser("u1", "alice", "hash", "user"))
	require.NoError(t, db.CreateItem("i1", "lib1", "/a.mkv", "a", "video", "mkv", 1000, 120.0, 1920, 1080, "h264", "aac"))
	require.NoError(t, db.CreateItem("i2", "lib1", "/b.mkv", "b", "video", "mkv", 1000, 90.0, 1920, 1080, "h264", "aac"))

	require.NoError(t, db.SavePlaybackProgress("u1", "i1", 10.0, 120.0, 8.3))
	require.NoError(t, db.SavePlaybackProgress("u1", "i2", 20.0, 90.0, 22.2))
	require.NoError(t, db.SaveWatchHistory("u1", "i1", 120.0, 120.0, true))

	report, err := db.GetPlaybackReporting("u1")
	require.NoError(t, err)

	progress, ok := report["playbackProgress"].([]PlaybackProgress)
	require.True(t, ok)
	assert.Len(t, progress, 2)

	history, ok := report["watchHistory"].([]WatchHistory)
	require.True(t, ok)
	assert.Len(t, history, 1)
	assert.True(t, history[0].Watched)
}
