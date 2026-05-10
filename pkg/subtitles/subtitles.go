package subtitles

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/devuser/aetherstream/pkg/probe"
)

// Track represents a subtitle track available in a media file.
type Track struct {
	Index       int    `json:"index"`
	Codec       string `json:"codec"`
	Language    string `json:"language"`
	Title       string `json:"title,omitempty"`
	Format      string `json:"format"`
	IsExternal  bool   `json:"is_external"`
	ExternalPath string `json:"external_path,omitempty"`
}

// ExtractResult holds the path to an extracted subtitle file.
type ExtractResult struct {
	Path     string `json:"path"`
	Format   string `json:"format"`
	Language string `json:"language"`
}

// ListTracks returns all subtitle tracks for a media file, including external sidecar files.
func ListTracks(mediaPath string) ([]Track, error) {
	var tracks []Track

	// 1. Embedded tracks via ffprobe
	info, err := probe.Probe(mediaPath)
	if err == nil && info != nil {
		for i, sub := range info.Subtitles {
			tr := Track{
				Index:    i,
				Codec:    sub.Codec,
				Language: sub.Language,
				Format:   codecToFormat(sub.Codec),
				IsExternal: false,
			}
			tracks = append(tracks, tr)
		}
	}

	// 2. External sidecar files (same directory, same base name)
	dir := filepath.Dir(mediaPath)
	base := strings.TrimSuffix(filepath.Base(mediaPath), filepath.Ext(mediaPath))
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasPrefix(name, base+".") {
				continue
			}
			ext := strings.ToLower(filepath.Ext(name))
			if ext == ".srt" || ext == ".ass" || ext == ".ssa" || ext == ".vtt" {
				lang := parseLanguageFromFilename(name)
				tracks = append(tracks, Track{
					Index:        len(tracks),
					Codec:        extToCodec(ext),
					Language:     lang,
					Format:       extToFormat(ext),
					IsExternal:   true,
					ExternalPath: filepath.Join(dir, name),
				})
			}
		}
	}

	return tracks, nil
}

// Extract extracts a subtitle track to a file.
// For embedded tracks, uses ffmpeg. For external tracks, returns the existing path.
func Extract(mediaPath string, trackIndex int, outDir string) (*ExtractResult, error) {
	tracks, err := ListTracks(mediaPath)
	if err != nil {
		return nil, err
	}
	if trackIndex < 0 || trackIndex >= len(tracks) {
		return nil, fmt.Errorf("track index %d out of range", trackIndex)
	}
	track := tracks[trackIndex]

	if track.IsExternal {
		return &ExtractResult{
			Path:     track.ExternalPath,
			Format:   track.Format,
			Language: track.Language,
		}, nil
	}

	_ = os.MkdirAll(outDir, 0750)
	outPath := filepath.Join(outDir, fmt.Sprintf("sub_%d_%s.%s", trackIndex, track.Language, track.Format))

	// ffmpeg extract embedded subtitle stream
	// #nosec G204 - mediaPath validated by caller; outPath is constructed within outDir
	cmd := exec.Command("ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-i", mediaPath,
		"-map", fmt.Sprintf("0:s:%d", track.Index),
		"-y",
		outPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg subtitle extract failed: %w\n%s", err, output)
	}

	return &ExtractResult{
		Path:     outPath,
		Format:   track.Format,
		Language: track.Language,
	}, nil
}

// ConvertToWebVTT converts a subtitle file (SRT/ASS/VTT) to WebVTT using ffmpeg.
func ConvertToWebVTT(inputPath, outputPath string) error {
	// #nosec G204 - paths validated by caller
	cmd := exec.Command("ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-i", inputPath,
		"-f", "webvtt",
		"-y",
		outputPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg webvtt conversion failed: %w\n%s", err, output)
	}
	return nil
}

// ServeWebVTT extracts and converts a subtitle track to WebVTT, returning the WebVTT file path.
func ServeWebVTT(mediaPath string, trackIndex int, outDir string) (string, error) {
	extracted, err := Extract(mediaPath, trackIndex, outDir)
	if err != nil {
		return "", err
	}

	if extracted.Format == "vtt" {
		return extracted.Path, nil
	}

	vttPath := strings.TrimSuffix(extracted.Path, filepath.Ext(extracted.Path)) + ".vtt"
	if err := ConvertToWebVTT(extracted.Path, vttPath); err != nil {
		return "", err
	}
	return vttPath, nil
}

// SelectTrack picks the best subtitle track based on language preference.
// Returns -1 if no suitable track found.
func SelectTrack(tracks []Track, preferredLang string) int {
	if preferredLang == "" {
		if len(tracks) > 0 {
			return 0
		}
		return -1
	}
	preferredLang = strings.ToLower(preferredLang)
	for i, t := range tracks {
		if strings.ToLower(t.Language) == preferredLang {
			return i
		}
	}
	// Fallback: first track
	if len(tracks) > 0 {
		return 0
	}
	return -1
}

// FormatDuration converts seconds to WebVTT timestamp (HH:MM:SS.mmm).
func FormatDuration(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second))
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	ms := int(d.Milliseconds()) % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}

// helper: map codec name to subtitle format
func codecToFormat(codec string) string {
	switch strings.ToLower(codec) {
	case "subrip", "srt":
		return "srt"
	case "ass", "ssa":
		return "ass"
	case "webvtt", "vtt":
		return "vtt"
	case "mov_text":
		return "srt"
	default:
		return "srt"
	}
}

func extToCodec(ext string) string {
	switch ext {
	case ".srt":
		return "subrip"
	case ".ass", ".ssa":
		return "ass"
	case ".vtt":
		return "webvtt"
	default:
		return "subrip"
	}
}

func extToFormat(ext string) string {
	switch ext {
	case ".srt":
		return "srt"
	case ".ass", ".ssa":
		return "ass"
	case ".vtt":
		return "vtt"
	default:
		return "srt"
	}
}

func parseLanguageFromFilename(name string) string {
	// Expects patterns like movie.en.srt, movie.eng.srt, movie.en-US.srt (exactly 3 parts)
	parts := strings.Split(name, ".")
	if len(parts) == 3 {
		candidate := strings.ToLower(parts[1])
		if len(candidate) >= 2 && len(candidate) <= 5 && isAlpha(candidate) {
			return candidate
		}
	}
	return "und"
}

func isAlpha(s string) bool {
	for _, r := range s {
		if (r < 'a' || r > 'z') && r != '-' {
			return false
		}
	}
	return true
}
