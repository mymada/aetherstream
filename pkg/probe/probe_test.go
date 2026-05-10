package probe

import (
	"testing"
)

func TestParseResultVideo(t *testing.T) {
	r := &FFProbeResult{
		Format: Format{
			FormatName: "matroska,webm",
			Duration:   "3600.5",
			BitRate:    "8000000",
			Size:       "3600000000",
			ProbeScore: 100,
		},
		Streams: []Stream{
			{
				CodecType:          "video",
				CodecName:          "h264",
				Width:              1920,
				Height:             1080,
				DisplayAspectRatio: "16:9",
				PixelFormat:        "yuv420p",
				BitRate:            "6000000",
				Duration:           "3600.5",
			},
		},
	}

	info := parseResult(r)
	if info.Format != "matroska,webm" {
		t.Errorf("Format = %q, want matroska,webm", info.Format)
	}
	if info.Duration != 3600.5 {
		t.Errorf("Duration = %f, want 3600.5", info.Duration)
	}
	if info.BitRate != 8000000 {
		t.Errorf("BitRate = %d, want 8000000", info.BitRate)
	}
	if info.Size != 3600000000 {
		t.Errorf("Size = %d, want 3600000000", info.Size)
	}
	if info.ProbeScore != 100 {
		t.Errorf("ProbeScore = %d, want 100", info.ProbeScore)
	}
	if info.Video == nil {
		t.Fatal("Video info missing")
	}
	if info.Video.Codec != "h264" {
		t.Errorf("Video.Codec = %q, want h264", info.Video.Codec)
	}
	if info.Video.Width != 1920 {
		t.Errorf("Video.Width = %d, want 1920", info.Video.Width)
	}
	if info.Video.Height != 1080 {
		t.Errorf("Video.Height = %d, want 1080", info.Video.Height)
	}
	if info.Video.BitRate != 6000000 {
		t.Errorf("Video.BitRate = %d, want 6000000", info.Video.BitRate)
	}
}

func TestParseResultAudio(t *testing.T) {
	r := &FFProbeResult{
		Format: Format{Duration: "180.0"},
		Streams: []Stream{
			{
				CodecType:     "audio",
				CodecName:     "aac",
				SampleRate:    "48000",
				Channels:      2,
				ChannelLayout: "stereo",
				BitRate:       "128000",
				Tags:          map[string]string{"language": "eng"},
			},
		},
	}

	info := parseResult(r)
	if info.Audio == nil {
		t.Fatal("Audio info missing")
	}
	if info.Audio.Codec != "aac" {
		t.Errorf("Audio.Codec = %q, want aac", info.Audio.Codec)
	}
	if info.Audio.SampleRate != 48000 {
		t.Errorf("Audio.SampleRate = %d, want 48000", info.Audio.SampleRate)
	}
	if info.Audio.Channels != 2 {
		t.Errorf("Audio.Channels = %d, want 2", info.Audio.Channels)
	}
	if info.Audio.ChannelLayout != "stereo" {
		t.Errorf("Audio.ChannelLayout = %q, want stereo", info.Audio.ChannelLayout)
	}
	if info.Audio.BitRate != 128000 {
		t.Errorf("Audio.BitRate = %d, want 128000", info.Audio.BitRate)
	}
	if info.Audio.Language != "eng" {
		t.Errorf("Audio.Language = %q, want eng", info.Audio.Language)
	}
}

func TestParseResultSubtitles(t *testing.T) {
	r := &FFProbeResult{
		Format: Format{},
		Streams: []Stream{
			{CodecType: "video", CodecName: "h264"},
			{CodecType: "subtitle", CodecName: "subrip", Tags: map[string]string{"language": "eng"}},
			{CodecType: "subtitle", CodecName: "ass", Tags: map[string]string{"language": "fre"}},
		},
	}

	info := parseResult(r)
	if len(info.Subtitles) != 2 {
		t.Fatalf("expected 2 subtitles, got %d", len(info.Subtitles))
	}
	if info.Subtitles[0].Codec != "subrip" || info.Subtitles[0].Language != "eng" {
		t.Errorf("first subtitle mismatch: %+v", info.Subtitles[0])
	}
	if info.Subtitles[1].Codec != "ass" || info.Subtitles[1].Language != "fre" {
		t.Errorf("second subtitle mismatch: %+v", info.Subtitles[1])
	}
}

func TestParseResultEmpty(t *testing.T) {
	r := &FFProbeResult{
		Format:  Format{},
		Streams: []Stream{},
	}

	info := parseResult(r)
	if info.Duration != 0 {
		t.Errorf("Duration = %f, want 0", info.Duration)
	}
	if info.BitRate != 0 {
		t.Errorf("BitRate = %d, want 0", info.BitRate)
	}
	if info.Video != nil {
		t.Error("expected no video stream")
	}
	if info.Audio != nil {
		t.Error("expected no audio stream")
	}
	if len(info.Subtitles) != 0 {
		t.Errorf("expected 0 subtitles, got %d", len(info.Subtitles))
	}
}

func TestParseVideoStream(t *testing.T) {
	s := &Stream{
		CodecName:          "hevc",
		Width:              3840,
		Height:             2160,
		DisplayAspectRatio: "16:9",
		PixelFormat:        "yuv420p10le",
		BitRate:            "15000000",
		Duration:           "7200.0",
	}
	v := parseVideoStream(s)
	if v.Codec != "hevc" {
		t.Errorf("Codec = %q, want hevc", v.Codec)
	}
	if v.Width != 3840 || v.Height != 2160 {
		t.Errorf("resolution = %dx%d, want 3840x2160", v.Width, v.Height)
	}
	if v.BitRate != 15000000 {
		t.Errorf("BitRate = %d, want 15000000", v.BitRate)
	}
	if v.Duration != 7200.0 {
		t.Errorf("Duration = %f, want 7200.0", v.Duration)
	}
}

func TestParseAudioStream(t *testing.T) {
	s := &Stream{
		CodecName:     "opus",
		SampleRate:    "48000",
		Channels:      6,
		ChannelLayout: "5.1",
		BitRate:       "256000",
		Tags:          map[string]string{"language": "jpn"},
	}
	a := parseAudioStream(s)
	if a.Codec != "opus" {
		t.Errorf("Codec = %q, want opus", a.Codec)
	}
	if a.SampleRate != 48000 {
		t.Errorf("SampleRate = %d, want 48000", a.SampleRate)
	}
	if a.Channels != 6 {
		t.Errorf("Channels = %d, want 6", a.Channels)
	}
	if a.ChannelLayout != "5.1" {
		t.Errorf("ChannelLayout = %q, want 5.1", a.ChannelLayout)
	}
	if a.BitRate != 256000 {
		t.Errorf("BitRate = %d, want 256000", a.BitRate)
	}
	if a.Language != "jpn" {
		t.Errorf("Language = %q, want jpn", a.Language)
	}
}
