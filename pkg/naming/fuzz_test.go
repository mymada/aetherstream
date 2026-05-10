package naming

import (
	"testing"
)

func FuzzParseFilename(f *testing.F) {
	// Seed corpus with typical and edge-case filenames
	seeds := []string{
		"The.Matrix.1999.1080p.mkv",
		"Show.Name.S01E02.mkv",
		"Show.Name.1x02.mkv",
		"01 - Artist - Title.mp3",
		"Movie Name (2024).mkv",
		"Movie.Name.(2024).mkv",
		"S01E02E03.mkv",
		"1x02-03.mkv",
		"",
		".mkv",
		"...",
		"S00E00.mkv",
		"Movie.1899.1080p.mkv",
		"Movie.2030.1080p.mkv",
		"Movie.2031.1080p.mkv",
		"999 - Track.mp3",
		"Artist - Album - 01 - Title.mp3",
		"/path/with/many/segments/Show.S02E05.720p.mkv",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, filename string) {
		// The function must not panic on any input.
		_ = ParseFilename(filename)
	})
}
