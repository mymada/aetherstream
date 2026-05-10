package encoder

import (
	"strings"
	"testing"
)

func TestGetProfileByName(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"audio_only", "audio_only"},
		{"mobile_low", "mobile_low"},
		{"mobile", "mobile"},
		{"tablet", "tablet"},
		{"tv", "tv"},
		{"tv_4k", "tv_4k"},
		{"unknown", "mobile"}, // default fallback
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := GetProfileByName(tt.name)
			if p.Name != tt.expected {
				t.Errorf("GetProfileByName(%q) = %q, want %q", tt.name, p.Name, tt.expected)
			}
		})
	}
}

func TestProfileCommandSoftware(t *testing.T) {
	p := ProfileMobile
	args := p.Command("/input.mp4", "/output.ts", "none")

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-i /input.mp4") {
		t.Error("missing input path")
	}
	if !strings.Contains(joined, "libx264") {
		t.Error("missing libx264 codec for software h264")
	}
	if !strings.Contains(joined, "-crf 23") {
		t.Error("missing CRF for software encoding")
	}
	if !strings.Contains(joined, "scale=1280:720") {
		t.Error("missing scale filter")
	}
	if !strings.Contains(joined, "-b:v 2000k") {
		t.Error("missing video bitrate")
	}
	if !strings.Contains(joined, "-b:a 128k") {
		t.Error("missing audio bitrate")
	}
}

func TestProfileCommandAudioOnly(t *testing.T) {
	p := ProfileAudioOnly
	args := p.Command("/input.mp4", "/output.aac", "none")

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-vn") {
		t.Error("audio-only profile should have -vn")
	}
	if strings.Contains(joined, "-c:v") {
		t.Error("audio-only profile should not set video codec")
	}
	if !strings.Contains(joined, "-c:a aac") {
		t.Error("missing aac audio codec")
	}
}

func TestProfileCommandHardwareAccel(t *testing.T) {
	p := ProfileTV // hevc
	args := p.Command("/input.mp4", "/output.ts", "nvenc")

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "hevc_nvenc") {
		t.Error("missing hevc_nvenc for nvenc hwaccel")
	}
	if strings.Contains(joined, "-crf") {
		t.Error("hardware encoding should not use CRF")
	}
}

func TestBuildHLSCommand(t *testing.T) {
	p := ProfileTablet
	args := BuildHLSCommand("/input.mkv", "/hls/out", p, 6, "none")

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-f hls") {
		t.Error("missing HLS format")
	}
	if !strings.Contains(joined, "-hls_time 6") {
		t.Error("missing segment duration")
	}
	if !strings.Contains(joined, "playlist.m3u8") {
		t.Error("missing playlist path")
	}
	if !strings.Contains(joined, "segment_%03d.ts") {
		t.Error("missing segment filename pattern")
	}
	if !strings.Contains(joined, "-hls_playlist_type vod") {
		t.Error("missing VOD playlist type")
	}
}
