package chapters

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Chapter represents a single chapter marker.
type Chapter struct {
	ID       int     `json:"id"`
	Title    string  `json:"title"`
	Start    float64 `json:"start"`
	End      float64 `json:"end"`
	Duration float64 `json:"duration"`
}

// FFProbeChapterResult represents ffprobe JSON output for chapters.
type FFProbeChapterResult struct {
	Chapters []FFProbeChapter `json:"chapters"`
	Format   Format           `json:"format"`
}

// FFProbeChapter represents a single chapter in ffprobe JSON output.
type FFProbeChapter struct {
	ID        int               `json:"id"`
	TimeBase  string            `json:"time_base"`
	Start     int64             `json:"start"`
	End       int64             `json:"end"`
	Tags      map[string]string `json:"tags"`
}

// Format container info (minimal for duration fallback).
type Format struct {
	Duration string `json:"duration"`
}

// ExtractChapters runs ffprobe to extract chapter markers from a media file.
func ExtractChapters(path string) ([]Chapter, error) {
	// #nosec G204 - path is validated by caller against library paths
	cmd := exec.Command(
		"ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_chapters",
		"-show_format",
		path,
	)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe chapters failed: %w", err)
	}

	var result FFProbeChapterResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("ffprobe chapters parse: %w", err)
	}

	if len(result.Chapters) == 0 {
		return []Chapter{}, nil
	}

	// Parse total duration from format for last chapter end fallback
	var totalDuration float64
	if d, err := strconv.ParseFloat(result.Format.Duration, 64); err == nil {
		totalDuration = d
	}

	chapters := make([]Chapter, 0, len(result.Chapters))
	for i, c := range result.Chapters {
		timeBase := ParseTimeBase(c.TimeBase)
		start := float64(c.Start) * timeBase
		end := float64(c.End) * timeBase

		// If end is zero or less than start, try to infer from next chapter start or total duration
		if end <= start {
			if i+1 < len(result.Chapters) {
				nextStart := float64(result.Chapters[i+1].Start) * timeBase
				if nextStart > start {
					end = nextStart
				}
			} else if totalDuration > start {
				end = totalDuration
			}
		}

		chapters = append(chapters, Chapter{
			ID:       c.ID,
			Title:    c.Tags["title"],
			Start:    start,
			End:      end,
			Duration: end - start,
		})
	}

	return chapters, nil
}

// ParseTimeBase parses an ffprobe time_base string (e.g. "1/1000") into seconds per unit.
func ParseTimeBase(tb string) float64 {
	if tb == "" {
		return 1.0 / 1000000000.0
	}
	parts := strings.Split(tb, "/")
	if len(parts) != 2 {
		return 1.0 / 1000000000.0
	}
	num, err1 := strconv.ParseFloat(parts[0], 64)
	den, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil || den == 0 {
		return 1.0 / 1000000000.0
	}
	return num / den
}

// FormatDuration formats seconds as HH:MM:SS or MM:SS.
func FormatDuration(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second))
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}
