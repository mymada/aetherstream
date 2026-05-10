package search

import (
	"testing"

	"github.com/devuser/aetherstream/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearcher_SearchItems(t *testing.T) {
	database, err := db.New(":memory:")
	require.NoError(t, err)
	defer database.Close()
	require.NoError(t, database.Migrate())

	// Seed library + items
	require.NoError(t, database.CreateLibrary("l1", "Movies", "/media/movies", "movie"))
	require.NoError(t, database.CreateItem("i1", "l1", "/media/movies/terminus.mp4", "Terminus", "movie", "mp4", 1000000, 3600, 1920, 1080, "h264", "aac"))
	require.NoError(t, database.CreateItem("i2", "l1", "/media/movies/alpha.mp4", "Alpha", "movie", "mp4", 2000000, 7200, 1920, 1080, "h264", "aac"))

	// Manually enrich FTS index (simulating metadata enrichment)
	require.NoError(t, database.UpdateFTSIndex("i1", "Terminus", "A sci-fi journey to the end of the line", "Alice Bob", "Director X"))
	require.NoError(t, database.UpdateFTSIndex("i2", "Alpha", "The beginning of everything", "Charlie", "Director Y"))

	searcher := NewSearcher(database)

	// Search by title
	results, err := searcher.SearchItems("Terminus", "", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "i1", results[0].ID)

	// Search by description
	results, err = searcher.SearchItems("beginning", "", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "i2", results[0].ID)

	// Search by actor
	results, err = searcher.SearchItems("Alice", "", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "i1", results[0].ID)

	// Search by director
	results, err = searcher.SearchItems("Director Y", "", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "i2", results[0].ID)

	// Filter by mediaType
	results, err = searcher.SearchItems("Alpha", "movie", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "i2", results[0].ID)

	// No match
	results, err = searcher.SearchItems("nonexistent", "", 10)
	require.NoError(t, err)
	assert.Len(t, results, 0)
}
