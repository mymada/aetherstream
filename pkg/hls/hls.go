package hls

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// PlaylistType represents HLS playlist variant
type PlaylistType string

const (
	PlaylistMaster  PlaylistType = "master"
	PlaylistVariant PlaylistType = "variant"
)

// Variant represents a quality variant in master playlist
type Variant struct {
	Bandwidth  int
	Resolution string
	Codecs     string
	PlaylistPath string
}

// MasterPlaylist generates HLS master.m3u8 content
func MasterPlaylist(variants []Variant) string {
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	b.WriteString("#EXT-X-VERSION:4\n")

	// Sort by bandwidth ascending
	sort.Slice(variants, func(i, j int) bool {
		return variants[i].Bandwidth < variants[j].Bandwidth
	})

	for _, v := range variants {
		b.WriteString(fmt.Sprintf(
			"#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%s,CODECS=\"%s\"\n",
			v.Bandwidth, v.Resolution, v.Codecs,
		))
		b.WriteString(v.PlaylistPath + "\n")
	}

	return b.String()
}

// VariantPlaylist generates a variant playlist from segments
func VariantPlaylist(segments []Segment, targetDuration int) string {
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	b.WriteString("#EXT-X-VERSION:4\n")
	b.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", targetDuration))
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	b.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	b.WriteString("#EXT-X-INDEPENDENT-SEGMENTS\n")

	for _, seg := range segments {
		b.WriteString(fmt.Sprintf("#EXTINF:%.3f,\n", seg.Duration))
		b.WriteString(seg.Path + "\n")
	}

	b.WriteString("#EXT-X-ENDLIST\n")
	return b.String()
}

// Segment represents an HLS segment
type Segment struct {
	Path     string
	Duration float64
	Size     int64
}

// ScanSegments discovers .ts segments in a directory
func ScanSegments(dir string) ([]Segment, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var segments []Segment
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".ts" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Parse duration from filename if possible (segment_001.ts)
		duration := 4.0 // default segment duration

		segments = append(segments, Segment{
			Path:     entry.Name(),
			Duration: duration,
			Size:     info.Size(),
		})
	}

	// Sort by filename
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].Path < segments[j].Path
	})

	return segments, nil
}

// WritePlaylist writes playlist content to file
func WritePlaylist(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// GenerateMasterForProfiles creates master playlist for predefined profiles
func GenerateMasterForProfiles(basePath string, profiles []string) string {
	variants := []Variant{}

	profileMap := map[string]struct {
		Bandwidth  int
		Resolution string
		Codecs     string
	}{
		"audio_only": {Bandwidth: 128000, Resolution: "0x0", Codecs: "mp4a.40.2"},
		"mobile_low": {Bandwidth: 800000, Resolution: "854x480", Codecs: "avc1.42e01e,mp4a.40.2"},
		"mobile":     {Bandwidth: 2000000, Resolution: "1280x720", Codecs: "avc1.64001f,mp4a.40.2"},
		"tablet":     {Bandwidth: 4000000, Resolution: "1920x1080", Codecs: "avc1.640028,mp4a.40.2"},
		"tv":         {Bandwidth: 6000000, Resolution: "1920x1080", Codecs: "hev1.1.6.L93.B0,mp4a.40.2"},
		"tv_4k":      {Bandwidth: 15000000, Resolution: "3840x2160", Codecs: "hev1.2.4.L153.B0,mp4a.40.2"},
	}

	for _, p := range profiles {
		if info, ok := profileMap[p]; ok {
			variants = append(variants, Variant{
				Bandwidth:    info.Bandwidth,
				Resolution:   info.Resolution,
				Codecs:       info.Codecs,
				PlaylistPath: p + "/playlist.m3u8",
			})
		}
	}

	return MasterPlaylist(variants)
}

// ParseTargetDuration extracts target duration from variant playlist
func ParseTargetDuration(content string) int {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "#EXT-X-TARGETDURATION:") {
			val := strings.TrimPrefix(line, "#EXT-X-TARGETDURATION:")
			if d, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
				return d
			}
		}
	}
	return 4 // default
}

// GetPlaylistDuration calculates total duration from variant playlist
func GetPlaylistDuration(content string) float64 {
	var total float64
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "#EXTINF:") {
			val := strings.TrimPrefix(line, "#EXTINF:")
			val = strings.TrimSuffix(val, ",")
			if d, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
				total += d
			}
		}
	}
	return total
}

// ExpiresAt returns when a live playlist should be refreshed
func ExpiresAt(lastModified time.Time, targetDuration int) time.Time {
	return lastModified.Add(time.Duration(targetDuration*2) * time.Second)
}
