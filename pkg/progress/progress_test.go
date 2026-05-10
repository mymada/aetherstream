package progress

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devuser/aetherstream/pkg/db"
)

func setupTestDB(t *testing.T) *db.DB {
	d, err := db.New(":memory:")
	require.NoError(t, err)
	require.NoError(t, d.Migrate())
	require.NoError(t, d.CreateUser("u1", "alice", "hash", "user"))
	require.NoError(t, d.CreateItem("i1", "lib1", "/a.mkv", "a", "video", "mkv", 1000, 120.0, 1920, 1080, "h264", "aac"))
	require.NoError(t, d.CreateItem("i2", "lib1", "/b.mkv", "b", "video", "mkv", 1000, 90.0, 1920, 1080, "h264", "aac"))
	return d
}

func TestSaveAndGetProgress(t *testing.T) {
	d := setupTestDB(t)
	defer d.Close()

	svc := NewService(d.DB)

	// Save progress
	require.NoError(t, svc.SaveProgress("u1", "i1", 45.5, 120.0, 37.9))

	// Get progress
	r, err := svc.GetProgress("u1", "i1")
	require.NoError(t, err)
	assert.Equal(t, "u1", r.UserID)
	assert.Equal(t, "i1", r.ItemID)
	assert.InDelta(t, 45.5, r.PositionSeconds, 0.001)
	assert.InDelta(t, 120.0, r.DurationSeconds, 0.001)
	assert.InDelta(t, 37.9, r.PercentComplete, 0.001)

	// Update progress
	require.NoError(t, svc.SaveProgress("u1", "i1", 90.0, 120.0, 75.0))
	r, err = svc.GetProgress("u1", "i1")
	require.NoError(t, err)
	assert.InDelta(t, 90.0, r.PositionSeconds, 0.001)
}

func TestDeleteProgress(t *testing.T) {
	d := setupTestDB(t)
	defer d.Close()

	svc := NewService(d.DB)
	require.NoError(t, svc.SaveProgress("u1", "i1", 45.0, 120.0, 37.5))

	require.NoError(t, svc.DeleteProgress("u1", "i1"))
	_, err := svc.GetProgress("u1", "i1")
	assert.Error(t, err)
}

func TestListProgressByUser(t *testing.T) {
	d := setupTestDB(t)
	defer d.Close()

	svc := NewService(d.DB)
	require.NoError(t, svc.SaveProgress("u1", "i1", 10.0, 120.0, 8.3))
	require.NoError(t, svc.SaveProgress("u1", "i2", 20.0, 90.0, 22.2))

	records, err := svc.ListProgressByUser("u1", 10)
	require.NoError(t, err)
	assert.Len(t, records, 2)
	assert.Equal(t, "i1", records[0].ItemID) // newest first
}

func TestMarkWatched(t *testing.T) {
	d := setupTestDB(t)
	defer d.Close()

	svc := NewService(d.DB)
	require.NoError(t, svc.SaveProgress("u1", "i1", 45.0, 120.0, 37.5))

	// Mark as watched (fully watched)
	require.NoError(t, svc.MarkWatched("u1", "i1", 120.0, 120.0))

	w, err := svc.GetWatchEntry("u1", "i1")
	require.NoError(t, err)
	assert.True(t, w.Watched)
	assert.InDelta(t, 120.0, w.PositionSeconds, 0.001)

	// Resume point should be cleared because fully watched
	_, err = svc.GetProgress("u1", "i1")
	assert.Error(t, err)
}

func TestMarkUnwatched(t *testing.T) {
	d := setupTestDB(t)
	defer d.Close()

	svc := NewService(d.DB)
	require.NoError(t, svc.MarkWatched("u1", "i1", 120.0, 120.0))
	require.NoError(t, svc.SaveProgress("u1", "i1", 45.0, 120.0, 37.5))

	// Mark unwatched
	require.NoError(t, svc.MarkUnwatched("u1", "i1"))

	w, err := svc.GetWatchEntry("u1", "i1")
	require.NoError(t, err)
	assert.False(t, w.Watched)
	assert.InDelta(t, 0.0, w.PositionSeconds, 0.001)

	// Resume point should be cleared
	_, err = svc.GetProgress("u1", "i1")
	assert.Error(t, err)
}

func TestListWatchHistoryByUser(t *testing.T) {
	d := setupTestDB(t)
	defer d.Close()

	svc := NewService(d.DB)
	require.NoError(t, svc.MarkWatched("u1", "i1", 120.0, 120.0))
	require.NoError(t, svc.MarkWatched("u1", "i2", 90.0, 90.0))

	list, err := svc.ListWatchHistoryByUser("u1", 10)
	require.NoError(t, err)
	assert.Len(t, list, 2)
	assert.True(t, list[0].Watched)
}

func TestSyncReport(t *testing.T) {
	d := setupTestDB(t)
	defer d.Close()

	svc := NewService(d.DB)
	require.NoError(t, svc.SaveProgress("u1", "i1", 10.0, 120.0, 8.3))
	require.NoError(t, svc.SaveProgress("u1", "i2", 20.0, 90.0, 22.2))
	require.NoError(t, svc.MarkWatched("u1", "i1", 120.0, 120.0))

	report, err := svc.SyncReport("u1")
	require.NoError(t, err)

	progress, ok := report["playbackProgress"].([]Record)
	require.True(t, ok)
	assert.Len(t, progress, 1)

	history, ok := report["watchHistory"].([]WatchEntry)
	require.True(t, ok)
	assert.Len(t, history, 1)
	assert.True(t, history[0].Watched)
}

func TestValidationErrors(t *testing.T) {
	d := setupTestDB(t)
	defer d.Close()

	svc := NewService(d.DB)

	// Missing IDs
	assert.Error(t, svc.SaveProgress("", "i1", 10, 120, 8))
	assert.Error(t, svc.SaveProgress("u1", "", 10, 120, 8))
	_, err := svc.GetProgress("", "i1")
	assert.Error(t, err)
	assert.Error(t, svc.DeleteProgress("u1", ""))
	assert.Error(t, svc.MarkWatched("", "i1", 120, 120))
	assert.Error(t, svc.MarkUnwatched("u1", ""))
	_, err = svc.ListProgressByUser("", 10)
	assert.Error(t, err)
	_, err = svc.ListWatchHistoryByUser("", 10)
	assert.Error(t, err)
	_, err = svc.SyncReport("")
	assert.Error(t, err)

	// Invalid duration
	assert.Error(t, svc.SaveProgress("u1", "i1", 10, 0, 8))
	assert.Error(t, svc.MarkWatched("u1", "i1", 10, 0))
}

func TestNegativePositionClamped(t *testing.T) {
	d := setupTestDB(t)
	defer d.Close()

	svc := NewService(d.DB)
	require.NoError(t, svc.SaveProgress("u1", "i1", -5.0, 120.0, -10.0))

	r, err := svc.GetProgress("u1", "i1")
	require.NoError(t, err)
	assert.InDelta(t, 0.0, r.PositionSeconds, 0.001)
	assert.InDelta(t, 0.0, r.PercentComplete, 0.001)
}

func TestPercentCompleteCapped(t *testing.T) {
	d := setupTestDB(t)
	defer d.Close()

	svc := NewService(d.DB)
	require.NoError(t, svc.SaveProgress("u1", "i1", 10.0, 120.0, 150.0))

	r, err := svc.GetProgress("u1", "i1")
	require.NoError(t, err)
	assert.InDelta(t, 100.0, r.PercentComplete, 0.001)
}
