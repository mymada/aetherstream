package hls

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMasterPlaylist(t *testing.T) {
	variants := []Variant{
		{Bandwidth: 2000000, Resolution: "1280x720", Codecs: "avc1.64001f,mp4a.40.2", PlaylistPath: "mobile/playlist.m3u8"},
		{Bandwidth: 800000, Resolution: "854x480", Codecs: "avc1.42e01e,mp4a.40.2", PlaylistPath: "mobile_low/playlist.m3u8"},
	}

	playlist := MasterPlaylist(variants)
	if !strings.HasPrefix(playlist, "#EXTM3U\n") {
		t.Error("master playlist missing EXTM3U header")
	}
	if !strings.Contains(playlist, "#EXT-X-STREAM-INF:BANDWIDTH=800000") {
		t.Error("master playlist missing low bandwidth variant")
	}
	if !strings.Contains(playlist, "#EXT-X-STREAM-INF:BANDWIDTH=2000000") {
		t.Error("master playlist missing high bandwidth variant")
	}
	// Should be sorted ascending by bandwidth
	idxLow := strings.Index(playlist, "800000")
	idxHigh := strings.Index(playlist, "2000000")
	if idxLow == -1 || idxHigh == -1 || idxLow > idxHigh {
		t.Error("variants not sorted by bandwidth ascending")
	}
}

func TestVariantPlaylist(t *testing.T) {
	segments := []Segment{
		{Path: "segment_001.ts", Duration: 4.0},
		{Path: "segment_002.ts", Duration: 4.0},
		{Path: "segment_003.ts", Duration: 3.5},
	}

	playlist := VariantPlaylist(segments, 6)
	if !strings.HasPrefix(playlist, "#EXTM3U\n") {
		t.Error("variant playlist missing EXTM3U header")
	}
	if !strings.Contains(playlist, "#EXT-X-TARGETDURATION:6") {
		t.Error("missing target duration")
	}
	if !strings.Contains(playlist, "#EXTINF:4.000,") {
		t.Error("missing segment duration")
	}
	if !strings.Contains(playlist, "#EXT-X-ENDLIST") {
		t.Error("missing ENDLIST tag")
	}
	if !strings.Contains(playlist, "segment_002.ts") {
		t.Error("missing segment path")
	}
}

func TestScanSegments(t *testing.T) {
	dir := t.TempDir()
	files := []string{"segment_001.ts", "segment_002.ts", "segment_003.ts", "playlist.m3u8"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("dummy"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	segs, err := ScanSegments(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 3 {
		t.Errorf("expected 3 segments, got %d", len(segs))
	}
	// Should be sorted
	if segs[0].Path != "segment_001.ts" {
		t.Errorf("first segment = %s, want segment_001.ts", segs[0].Path)
	}
	if segs[2].Path != "segment_003.ts" {
		t.Errorf("last segment = %s, want segment_003.ts", segs[2].Path)
	}
}

func TestWritePlaylist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.m3u8")
	content := "#EXTM3U\n#EXT-X-ENDLIST\n"
	if err := WritePlaylist(path, content); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Errorf("written content mismatch: got %q, want %q", string(data), content)
	}
}

func TestGenerateMasterForProfiles(t *testing.T) {
	profiles := []string{"mobile_low", "mobile", "tv_4k"}
	playlist := GenerateMasterForProfiles("/hls", profiles)
	if !strings.Contains(playlist, "mobile_low/playlist.m3u8") {
		t.Error("missing mobile_low variant")
	}
	if !strings.Contains(playlist, "mobile/playlist.m3u8") {
		t.Error("missing mobile variant")
	}
	if !strings.Contains(playlist, "tv_4k/playlist.m3u8") {
		t.Error("missing tv_4k variant")
	}
	if strings.Contains(playlist, "audio_only") {
		t.Error("audio_only should not appear")
	}
}

func TestParseTargetDuration(t *testing.T) {
	content := "#EXTM3U\n#EXT-X-TARGETDURATION:8\n#EXTINF:4.0,\nseg.ts\n"
	if d := ParseTargetDuration(content); d != 8 {
		t.Errorf("ParseTargetDuration = %d, want 8", d)
	}
	if d := ParseTargetDuration("no tag here"); d != 4 {
		t.Errorf("default target duration = %d, want 4", d)
	}
}

func TestGetPlaylistDuration(t *testing.T) {
	content := "#EXTM3U\n#EXTINF:4.000,\nseg1.ts\n#EXTINF:3.500,\nseg2.ts\n#EXTINF:2.000,\nseg3.ts\n"
	if d := GetPlaylistDuration(content); d != 9.5 {
		t.Errorf("GetPlaylistDuration = %f, want 9.5", d)
	}
	if d := GetPlaylistDuration(""); d != 0 {
		t.Errorf("empty duration = %f, want 0", d)
	}
}

func TestExpiresAt(t *testing.T) {
	now := time.Now()
	exp := ExpiresAt(now, 5)
	expected := now.Add(10 * time.Second)
	if exp.Sub(expected).Abs() > time.Millisecond {
		t.Errorf("ExpiresAt mismatch")
	}
}
