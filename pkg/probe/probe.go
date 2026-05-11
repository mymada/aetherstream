package probe

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// FFProbeResult represents ffprobe JSON output
type FFProbeResult struct {
	Format  Format   `json:"format"`
	Streams []Stream `json:"streams"`
}

// Format container info
type Format struct {
	Filename       string `json:"filename"`
	FormatName     string `json:"format_name"`
	Duration       string `json:"duration"`
	BitRate        string `json:"bit_rate"`
	Size           string `json:"size"`
	ProbeScore     int    `json:"probe_score"`
	Tags           map[string]string `json:"tags"`
}

// Disposition holds ffprobe stream disposition flags
type Disposition struct {
	Default int `json:"default"`
	Forced  int `json:"forced"`
}

// Stream represents a media stream (video/audio/subtitle)
type Stream struct {
	Index          int    `json:"index"`
	CodecName      string `json:"codec_name"`
	CodecLongName  string `json:"codec_long_name"`
	CodecType      string `json:"codec_type"`
	CodecTagString string `json:"codec_tag_string"`
	Width          int    `json:"width,omitempty"`
	Height         int    `json:"height,omitempty"`
	DisplayAspectRatio string `json:"display_aspect_ratio,omitempty"`
	PixelFormat    string `json:"pix_fmt,omitempty"`
	BitRate        string `json:"bit_rate,omitempty"`
	SampleRate     string `json:"sample_rate,omitempty"`
	Channels       int    `json:"channels,omitempty"`
	ChannelLayout  string `json:"channel_layout,omitempty"`
	Duration       string `json:"duration,omitempty"`
	Language       string `json:"language,omitempty"`
	Tags           map[string]string `json:"tags"`
	Disposition    Disposition       `json:"disposition"`
}

// MediaInfo is the processed result
type MediaInfo struct {
	Duration    float64
	BitRate     int64
	Size        int64
	Format      string
	Video       *VideoInfo
	Audio       *AudioInfo   // first/default audio stream (backward compat)
	AllAudio    []AudioInfo  // all audio streams
	Subtitles   []SubtitleInfo
	ProbeScore  int
}

// VideoInfo extracted video properties
type VideoInfo struct {
	Codec        string
	Width        int
	Height       int
	AspectRatio  string
	PixelFormat  string
	BitRate      int64
	FrameRate    float64
	Duration     float64
}

// AudioInfo extracted audio properties
type AudioInfo struct {
	StreamIndex   int
	SubIndex      int    // index within audio streams (for -map 0:a:N)
	Codec         string
	SampleRate    int
	Channels      int
	ChannelLayout string
	BitRate       int64
	Language      string
	Title         string
	Default       bool
}

// SubtitleInfo describes a single subtitle stream
type SubtitleInfo struct {
	StreamIndex int
	SubIndex    int    // index within subtitle streams (for -map 0:s:N)
	Codec       string
	Language    string
	Title       string
	Forced      bool
	Default     bool
}

// Probe runs ffprobe on a media file
func Probe(path string) (*MediaInfo, error) {
	// #nosec G204 - path is validated by caller against library paths
	cmd := exec.Command(
		"ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var result FFProbeResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("ffprobe parse: %w", err)
	}

	return parseResult(&result), nil
}

func parseResult(r *FFProbeResult) *MediaInfo {
	info := &MediaInfo{
		Format:     r.Format.FormatName,
		ProbeScore: r.Format.ProbeScore,
	}

	// Parse format fields
	if d, err := strconv.ParseFloat(r.Format.Duration, 64); err == nil {
		info.Duration = d
	}
	if br, err := strconv.ParseInt(r.Format.BitRate, 10, 64); err == nil {
		info.BitRate = br
	}
	if sz, err := strconv.ParseInt(r.Format.Size, 10, 64); err == nil {
		info.Size = sz
	}

	// Parse streams — track per-type counters for -map 0:a:N / 0:s:N indexing
	audioIdx := 0
	subIdx := 0
	for _, s := range r.Streams {
		switch s.CodecType {
		case "video":
			info.Video = parseVideoStream(&s)
		case "audio":
			a := parseAudioStream(&s)
			a.StreamIndex = s.Index
			a.SubIndex = audioIdx
			a.Title = s.Tags["title"]
			a.Default = s.Disposition.Default == 1
			info.AllAudio = append(info.AllAudio, *a)
			if info.Audio == nil {
				info.Audio = a
			}
			audioIdx++
		case "subtitle":
			info.Subtitles = append(info.Subtitles, SubtitleInfo{
				StreamIndex: s.Index,
				SubIndex:    subIdx,
				Codec:       s.CodecName,
				Language:    s.Tags["language"],
				Title:       s.Tags["title"],
				Forced:      s.Disposition.Forced == 1,
				Default:     s.Disposition.Default == 1,
			})
			subIdx++
		}
	}

	return info
}

func parseVideoStream(s *Stream) *VideoInfo {
	v := &VideoInfo{
		Codec:       s.CodecName,
		Width:       s.Width,
		Height:      s.Height,
		AspectRatio: s.DisplayAspectRatio,
		PixelFormat: s.PixelFormat,
	}

	if br, err := strconv.ParseInt(s.BitRate, 10, 64); err == nil {
		v.BitRate = br
	}
	if d, err := strconv.ParseFloat(s.Duration, 64); err == nil {
		v.Duration = d
	}

	return v
}

func parseAudioStream(s *Stream) *AudioInfo {
	a := &AudioInfo{
		Codec:         s.CodecName,
		Channels:      s.Channels,
		ChannelLayout: s.ChannelLayout,
		Language:      s.Tags["language"],
	}

	if sr, err := strconv.Atoi(s.SampleRate); err == nil {
		a.SampleRate = sr
	}
	if br, err := strconv.ParseInt(s.BitRate, 10, 64); err == nil {
		a.BitRate = br
	}

	return a
}

// ExtractSubtitleByIndex extracts a subtitle stream by its subtitle-relative index (0:s:N).
func ExtractSubtitleByIndex(path string, subIndex int) (string, error) {
	if subIndex < 0 || subIndex > 99 {
		return "", fmt.Errorf("invalid subtitle index: %d", subIndex)
	}

	f, err := os.CreateTemp("", "aetherstream_sub_*.srt")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	outPath := f.Name()
	_ = f.Close()

	// #nosec G204 - path validated by caller; outPath is a secure temp file
	cmd := exec.Command("ffmpeg", "-i", path, "-map", fmt.Sprintf("0:s:%d", subIndex), outPath, "-y")
	if _, err := cmd.CombinedOutput(); err != nil {
		os.Remove(outPath)
		return "", fmt.Errorf("subtitle extraction failed: %w", err)
	}
	return outPath, nil
}

// ExtractSubtitleTracks returns all subtitle streams from a media file with full metadata.
func ExtractSubtitleTracks(path string) ([]map[string]interface{}, error) {
	info, err := Probe(path)
	if err != nil {
		return nil, err
	}
	var tracks []map[string]interface{}
	for _, sub := range info.Subtitles {
		tracks = append(tracks, map[string]interface{}{
			"sub_index":    sub.SubIndex,
			"stream_index": sub.StreamIndex,
			"codec":        sub.Codec,
			"language":     sub.Language,
			"title":        sub.Title,
			"forced":       sub.Forced,
			"default":      sub.Default,
		})
	}
	return tracks, nil
}

// ExtractSubtitleToFile extracts a subtitle stream to SRT file using secure temp file
func ExtractSubtitleToFile(path, lang string) (string, error) {
	// Validate language code
	if !isValidLanguageCode(lang) {
		return "", fmt.Errorf("invalid language code: %s", lang)
	}

	// Find subtitle index for language
	info, err := Probe(path)
	if err != nil {
		return "", err
	}
	var subIndex int = -1
	for i, sub := range info.Subtitles {
		if sub.Language == lang {
			subIndex = i
			break
		}
	}
	if subIndex == -1 {
		return "", fmt.Errorf("subtitle language %s not found", lang)
	}

	// Use secure temp file (fixes M4: no fixed path, no collision, no symlink attack)
	f, err := os.CreateTemp("", "aetherstream_sub_*.srt")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	outPath := f.Name()
	_ = f.Close()

	// #nosec G204 - path is validated by caller against library paths; outPath is secure temp
	cmd := exec.Command("ffmpeg", "-i", path, "-map", fmt.Sprintf("0:s:%d", subIndex), outPath, "-y")
	if _, err := cmd.CombinedOutput(); err != nil {
		os.Remove(outPath)
		return "", fmt.Errorf("subtitle extraction failed: %w", err)
	}
	return outPath, nil
}

// isValidLanguageCode validates ISO 639-1/2 language codes
func isValidLanguageCode(lang string) bool {
	if lang == "" {
		return false
	}
	for _, r := range lang {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '-') {
			return false
		}
	}
	return true
}
