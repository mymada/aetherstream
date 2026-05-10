package subtitles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds float64
		want    string
	}{
		{0, "00:00:00.000"},
		{1.5, "00:00:01.500"},
		{61.123, "00:01:01.123"},
		{3661.456, "01:01:01.456"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, FormatDuration(tt.seconds))
	}
}

func TestSelectTrack(t *testing.T) {
	tracks := []Track{
		{Index: 0, Language: "eng", Format: "srt"},
		{Index: 1, Language: "fre", Format: "vtt"},
		{Index: 2, Language: "spa", Format: "ass"},
	}

	assert.Equal(t, 0, SelectTrack(tracks, "eng"))
	assert.Equal(t, 1, SelectTrack(tracks, "fre"))
	assert.Equal(t, 2, SelectTrack(tracks, "spa"))
	assert.Equal(t, 0, SelectTrack(tracks, ""))      // fallback first
	assert.Equal(t, 0, SelectTrack(tracks, "ger"))    // fallback first
	assert.Equal(t, -1, SelectTrack([]Track{}, "eng"))
}

func TestCodecToFormat(t *testing.T) {
	assert.Equal(t, "srt", codecToFormat("subrip"))
	assert.Equal(t, "srt", codecToFormat("srt"))
	assert.Equal(t, "ass", codecToFormat("ass"))
	assert.Equal(t, "vtt", codecToFormat("webvtt"))
	assert.Equal(t, "srt", codecToFormat("mov_text"))
	assert.Equal(t, "srt", codecToFormat("unknown"))
}

func TestExtToFormat(t *testing.T) {
	assert.Equal(t, "srt", extToFormat(".srt"))
	assert.Equal(t, "ass", extToFormat(".ass"))
	assert.Equal(t, "ass", extToFormat(".ssa"))
	assert.Equal(t, "vtt", extToFormat(".vtt"))
	assert.Equal(t, "srt", extToFormat(".xyz"))
}

func TestParseLanguageFromFilename(t *testing.T) {
	assert.Equal(t, "en", parseLanguageFromFilename("movie.en.srt"))
	assert.Equal(t, "eng", parseLanguageFromFilename("movie.eng.srt"))
	assert.Equal(t, "en-us", parseLanguageFromFilename("movie.en-us.srt"))
	assert.Equal(t, "und", parseLanguageFromFilename("movie.srt"))
	assert.Equal(t, "und", parseLanguageFromFilename("movie.something.very.long.srt"))
	assert.Equal(t, "und", parseLanguageFromFilename("movie.1234.srt"))
}

func TestListTracksExternal(t *testing.T) {
	tmpDir := t.TempDir()
	mediaPath := filepath.Join(tmpDir, "movie.mp4")
	require.NoError(t, os.WriteFile(mediaPath, []byte("fake"), 0644))

	// Create external sidecar subtitles
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "movie.en.srt"), []byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "movie.fr.vtt"), []byte("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nBonjour\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "movie.ass"), []byte("[Script Info]\n"), 0644)) // wrong base name

	tracks, err := ListTracks(mediaPath)
	require.NoError(t, err)

	// We should get at least the two sidecar tracks (ffprobe may fail on fake mp4)
	var externalCount int
	for _, tr := range tracks {
		if tr.IsExternal {
			externalCount++
		}
	}
	assert.GreaterOrEqual(t, externalCount, 2)
}

func TestExtractExternalTrack(t *testing.T) {
	tmpDir := t.TempDir()
	mediaPath := filepath.Join(tmpDir, "movie.mp4")
	require.NoError(t, os.WriteFile(mediaPath, []byte("fake"), 0644))

	srtPath := filepath.Join(tmpDir, "movie.en.srt")
	require.NoError(t, os.WriteFile(srtPath, []byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n"), 0644))

	tracks, err := ListTracks(mediaPath)
	require.NoError(t, err)

	// Find external track
	var extIdx int = -1
	for i, tr := range tracks {
		if tr.IsExternal {
			extIdx = i
			break
		}
	}
	require.GreaterOrEqual(t, extIdx, 0, "expected at least one external track")

	outDir := filepath.Join(tmpDir, "subs")
	res, err := Extract(mediaPath, extIdx, outDir)
	require.NoError(t, err)
	assert.Equal(t, srtPath, res.Path)
	assert.Equal(t, "srt", res.Format)
}

func TestExtractOutOfRange(t *testing.T) {
	tmpDir := t.TempDir()
	mediaPath := filepath.Join(tmpDir, "movie.mp4")
	require.NoError(t, os.WriteFile(mediaPath, []byte("fake"), 0644))

	_, err := Extract(mediaPath, 99, tmpDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestConvertToWebVTT(t *testing.T) {
	tmpDir := t.TempDir()
	srtPath := filepath.Join(tmpDir, "test.srt")
	vttPath := filepath.Join(tmpDir, "test.vtt")

	require.NoError(t, os.WriteFile(srtPath, []byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n"), 0644))

	// ffmpeg may not be available in test environment; skip if not found
	if _, err := os.Stat("/usr/bin/ffmpeg"); os.IsNotExist(err) {
		if _, err2 := os.Stat("/usr/local/bin/ffmpeg"); os.IsNotExist(err2) {
			t.Skip("ffmpeg not available")
		}
	}

	err := ConvertToWebVTT(srtPath, vttPath)
	require.NoError(t, err)

	data, err := os.ReadFile(vttPath)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(data), "WEBVTT"))
}

func TestServeWebVTTExternal(t *testing.T) {
	tmpDir := t.TempDir()
	mediaPath := filepath.Join(tmpDir, "movie.mp4")
	require.NoError(t, os.WriteFile(mediaPath, []byte("fake"), 0644))

	vttPath := filepath.Join(tmpDir, "movie.en.vtt")
	require.NoError(t, os.WriteFile(vttPath, []byte("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHello\n"), 0644))

	outDir := filepath.Join(tmpDir, "subs")
	res, err := ServeWebVTT(mediaPath, 0, outDir)
	require.NoError(t, err)
	assert.Equal(t, vttPath, res)
}

func TestServeWebVTTConvert(t *testing.T) {
	tmpDir := t.TempDir()
	mediaPath := filepath.Join(tmpDir, "movie.mp4")
	require.NoError(t, os.WriteFile(mediaPath, []byte("fake"), 0644))

	srtPath := filepath.Join(tmpDir, "movie.en.srt")
	require.NoError(t, os.WriteFile(srtPath, []byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n"), 0644))

	// ffmpeg may not be available
	if _, err := os.Stat("/usr/bin/ffmpeg"); os.IsNotExist(err) {
		if _, err2 := os.Stat("/usr/local/bin/ffmpeg"); os.IsNotExist(err2) {
			t.Skip("ffmpeg not available")
		}
	}

	outDir := filepath.Join(tmpDir, "subs")
	res, err := ServeWebVTT(mediaPath, 0, outDir)
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(res, ".vtt"))

	data, err := os.ReadFile(res)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(data), "WEBVTT"))
}
