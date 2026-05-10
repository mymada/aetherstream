package m3u

import (
	"fmt"
	"strings"
	"time"
)

// PlaylistEntry represents a single M3U entry.
type PlaylistEntry struct {
	Title    string
	Duration float64 // seconds, -1 for live streams
	URL      string
	Group    string // optional group-title
}

// ExportM3U generates an M3U playlist string from entries.
func ExportM3U(entries []PlaylistEntry) string {
	var sb strings.Builder
	sb.WriteString("#EXTM3U\n")
	for _, e := range entries {
		dur := e.Duration
		if dur < 0 {
			dur = -1
		}
		if e.Group != "" {
			sb.WriteString(fmt.Sprintf("#EXTINF:%.0f group-title=\"%s\",%s\n", dur, e.Group, e.Title))
		} else {
			sb.WriteString(fmt.Sprintf("#EXTINF:%.0f,%s\n", dur, e.Title))
		}
		sb.WriteString(e.URL + "\n")
	}
	return sb.String()
}

// ExportM3UWithMetadata generates an M3U playlist with additional metadata comments.
func ExportM3UWithMetadata(entries []PlaylistEntry, name string) string {
	var sb strings.Builder
	sb.WriteString("#EXTM3U\n")
	sb.WriteString(fmt.Sprintf("#PLAYLIST:%s\n", name))
	sb.WriteString(fmt.Sprintf("#EXT-X-PLAYLIST-TYPE:VOD\n"))
	sb.WriteString(fmt.Sprintf("#CREATED:%s\n", time.Now().UTC().Format(time.RFC3339)))
	for _, e := range entries {
		dur := e.Duration
		if dur < 0 {
			dur = -1
		}
		if e.Group != "" {
			sb.WriteString(fmt.Sprintf("#EXTINF:%.0f group-title=\"%s\",%s\n", dur, e.Group, e.Title))
		} else {
			sb.WriteString(fmt.Sprintf("#EXTINF:%.0f,%s\n", dur, e.Title))
		}
		sb.WriteString(e.URL + "\n")
	}
	return sb.String()
}
