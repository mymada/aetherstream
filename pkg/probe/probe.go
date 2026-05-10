package probe

import (
	"encoding/json"
	"fmt"
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
}

// MediaInfo is the processed result
type MediaInfo struct {
	Duration    float64
	BitRate     int64
	Size        int64
	Format      string
	Video       *VideoInfo
	Audio       *AudioInfo
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
	Codec        string
	SampleRate   int
	Channels     int
	ChannelLayout string
	BitRate      int64
	Language     string
}

// SubtitleInfo
type SubtitleInfo struct {
	Codec    string
	Language string
}

// Probe runs ffprobe on a media file
func Probe(path string) (*MediaInfo, error) {
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

	// Parse streams
	for _, s := range r.Streams {
		switch s.CodecType {
		case "video":
			info.Video = parseVideoStream(&s)
		case "audio":
			info.Audio = parseAudioStream(&s)
		case "subtitle":
			info.Subtitles = append(info.Subtitles, SubtitleInfo{
				Codec:    s.CodecName,
				Language: s.Tags["language"],
			})
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

// ExtractSubtitleTracks returns subtitle streams from a media file
func ExtractSubtitleTracks(path string) ([]map[string]interface{}, error) {
	info, err := Probe(path)
	if err != nil {
		return nil, err
	}
	var tracks []map[string]interface{}
	for i, sub := range info.Subtitles {
		tracks = append(tracks, map[string]interface{}{
			"index":    i,
			"codec":    sub.Codec,
			"language": sub.Language,
		})
	}
	return tracks, nil
}

// ExtractSubtitleToFile extracts a subtitle stream to SRT file
func ExtractSubtitleToFile(path, lang string) (string, error) {
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

	outPath := "/tmp/aetherstream_sub_" + lang + ".srt"
	cmd := exec.Command("ffmpeg", "-i", path, "-map", fmt.Sprintf("0:s:%d", subIndex), outPath, "-y")
	if _, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("subtitle extraction failed: %w", err)
	}
	return outPath, nil
}
