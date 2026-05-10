package naming

import (
	"testing"
)

func TestParseFilenameEpisode(t *testing.T) {
	r := ParseFilename("/media/Show.Name.S01E02.mkv")
	if r.Kind != "episode" {
		t.Errorf("Kind = %q, want episode", r.Kind)
	}
	if r.Season != 1 {
		t.Errorf("Season = %d, want 1", r.Season)
	}
	if r.Episode != 2 {
		t.Errorf("Episode = %d, want 2", r.Episode)
	}
	if r.Title != "Show Name" {
		t.Errorf("Title = %q, want 'Show Name'", r.Title)
	}
}

func TestParseFilenameEpisodeMulti(t *testing.T) {
	r := ParseFilename("Show.Name.S02E05E06.1080p.mkv")
	if r.Kind != "episode" {
		t.Errorf("Kind = %q, want episode", r.Kind)
	}
	if r.Season != 2 {
		t.Errorf("Season = %d, want 2", r.Season)
	}
	if r.Episode != 5 {
		t.Errorf("Episode = %d, want 5", r.Episode)
	}
	if r.EpisodeEnd != 6 {
		t.Errorf("EpisodeEnd = %d, want 6", r.EpisodeEnd)
	}
}

func TestParseFilenameEpisodeXFormat(t *testing.T) {
	r := ParseFilename("Show.Name.3x12.mkv")
	if r.Kind != "episode" {
		t.Errorf("Kind = %q, want episode", r.Kind)
	}
	if r.Season != 3 {
		t.Errorf("Season = %d, want 3", r.Season)
	}
	if r.Episode != 12 {
		t.Errorf("Episode = %d, want 12", r.Episode)
	}
}

func TestParseFilenameMovieDotYear(t *testing.T) {
	r := ParseFilename("/movies/The.Matrix.1999.1080p.mkv")
	if r.Kind != "movie" {
		t.Errorf("Kind = %q, want movie", r.Kind)
	}
	if r.Year != 1999 {
		t.Errorf("Year = %d, want 1999", r.Year)
	}
	if r.Title != "The Matrix" {
		t.Errorf("Title = %q, want 'The Matrix'", r.Title)
	}
}

func TestParseFilenameMovieParenYear(t *testing.T) {
	r := ParseFilename("Movie Name (2024).mkv")
	if r.Kind != "movie" {
		t.Errorf("Kind = %q, want movie", r.Kind)
	}
	if r.Year != 2024 {
		t.Errorf("Year = %d, want 2024", r.Year)
	}
	if r.Title != "Movie Name" {
		t.Errorf("Title = %q, want 'Movie Name'", r.Title)
	}
}

func TestParseFilenameMusic(t *testing.T) {
	r := ParseFilename("/music/01 - Artist - Title.mp3")
	if r.Kind != "music" {
		t.Errorf("Kind = %q, want music", r.Kind)
	}
	if r.TrackNumber != 1 {
		t.Errorf("TrackNumber = %d, want 1", r.TrackNumber)
	}
	if r.Title != "Artist - Title" {
		t.Errorf("Title = %q, want 'Artist - Title'", r.Title)
	}
}

func TestParseFilenameUnknown(t *testing.T) {
	r := ParseFilename("random_file.txt")
	if r.Kind != "unknown" {
		t.Errorf("Kind = %q, want unknown", r.Kind)
	}
	if r.Title != "random file" {
		t.Errorf("Title = %q, want 'random file'", r.Title)
	}
}

func TestCleanName(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Movie.Name.1080p.BluRay.x264", "Movie Name"},
		{"Show_Name_720p_HDTV", "Show Name"},
		{"Title.4K.HDR.DV", "Title"},
		{"Movie Name (2024)", "Movie Name (2024)"},
	}

	for _, c := range cases {
		got := cleanName(c.input)
		if got != c.expected {
			t.Errorf("cleanName(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}
