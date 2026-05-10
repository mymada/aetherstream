package nfo

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageCompiles(t *testing.T) {
	assert.True(t, true, "nfo package should compile")
}

func TestWriteReadMovieNFO(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "movie.nfo")
	movie := &Movie{
		Title:   "Test Movie",
		Year:    2024,
		Plot:    "A test plot.",
		Rating:  7.5,
		Runtime: 120,
		Genre:   []string{"Sci-Fi", "Action"},
		Actor:   []Actor{{Name: "Alice", Role: "Hero"}},
	}
	require.NoError(t, WriteMovieNFO(path, movie))
	read, err := ReadMovieNFO(path)
	require.NoError(t, err)
	assert.Equal(t, "Test Movie", read.Title)
	assert.Equal(t, 2024, read.Year)
	assert.Equal(t, "A test plot.", read.Plot)
}

func TestNFOPath(t *testing.T) {
	assert.Equal(t, "/media/movie.nfo", NFOPath("/media/movie.mkv"))
}

func TestWriteReadEpisodeNFO(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "episode.nfo")
	ep := &Episode{
		Title:   "Pilot",
		ShowTitle: "Test Show",
		Season:  1,
		Episode: 1,
		Plot:    "The beginning.",
	}
	require.NoError(t, WriteEpisodeNFO(path, ep))
	read, err := ReadEpisodeNFO(path)
	require.NoError(t, err)
	assert.Equal(t, "Pilot", read.Title)
	assert.Equal(t, 1, read.Season)
}

func TestSimpleMovieFromMap(t *testing.T) {
	m := SimpleMovieFromMap(map[string]interface{}{
		"name":            "Foo",
		"durationSeconds":   float64(7200),
		"width":           1920,
		"height":          1080,
	})
	assert.Equal(t, "Foo", m.Title)
	assert.Equal(t, 120, m.Runtime)
	require.NotNil(t, m.FileInfo)
	assert.Equal(t, 1920, m.FileInfo.StreamDetails.Video[0].Width)
	assert.Equal(t, 1080, m.FileInfo.StreamDetails.Video[0].Height)
}

func TestReadMissingNFO(t *testing.T) {
	_, err := ReadMovieNFO("/nonexistent/path.nfo")
	assert.Error(t, err)
}
