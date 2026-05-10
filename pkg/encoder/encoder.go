package encoder

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Profile defines a transcoding preset
type Profile struct {
	Name        string
	Width       int
	Height      int
	VideoCodec  string // h264, hevc, vp9
	AudioCodec  string // aac, opus, ac3
	VideoBitrate int   // kbps
	AudioBitrate int   // kbps
	Container   string // mp4, ts, webm
}

// Default profiles
var (
	ProfileAudioOnly = Profile{Name: "audio_only", VideoCodec: "", AudioCodec: "aac", AudioBitrate: 128}
	ProfileMobileLow = Profile{Name: "mobile_low", Width: 854, Height: 480, VideoCodec: "h264", AudioCodec: "aac", VideoBitrate: 800, AudioBitrate: 128, Container: "ts"}
	ProfileMobile    = Profile{Name: "mobile", Width: 1280, Height: 720, VideoCodec: "h264", AudioCodec: "aac", VideoBitrate: 2000, AudioBitrate: 128, Container: "ts"}
	ProfileTablet    = Profile{Name: "tablet", Width: 1920, Height: 1080, VideoCodec: "h264", AudioCodec: "aac", VideoBitrate: 4000, AudioBitrate: 192, Container: "ts"}
	ProfileTV        = Profile{Name: "tv", Width: 1920, Height: 1080, VideoCodec: "hevc", AudioCodec: "aac", VideoBitrate: 6000, AudioBitrate: 192, Container: "ts"}
	ProfileTV4K      = Profile{Name: "tv_4k", Width: 3840, Height: 2160, VideoCodec: "hevc", AudioCodec: "aac", VideoBitrate: 15000, AudioBitrate: 192, Container: "ts"}
)

// GetProfileByName returns a predefined profile
func GetProfileByName(name string) Profile {
	switch name {
	case "audio_only":
		return ProfileAudioOnly
	case "mobile_low":
		return ProfileMobileLow
	case "mobile":
		return ProfileMobile
	case "tablet":
		return ProfileTablet
	case "tv":
		return ProfileTV
	case "tv_4k":
		return ProfileTV4K
	default:
		return ProfileMobile
	}
}

// Command builds FFmpeg command args for a profile
func (p Profile) Command(inputPath, outputPath string, hwAccel string) []string {
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-i", inputPath,
		"-y",
	}

	// Hardware acceleration
	switch hwAccel {
	case "vaapi":
		args = append(args, "-hwaccel", "vaapi", "-vaapi_device", "/dev/dri/renderD128")
	case "nvenc":
		args = append(args, "-hwaccel", "cuda")
	case "qsv":
		args = append(args, "-hwaccel", "qsv")
	}

	// Video encoding
	if p.VideoCodec != "" {
		// Scale filter
		if p.Width > 0 && p.Height > 0 {
			args = append(args, "-vf", fmt.Sprintf("scale=%d:%d", p.Width, p.Height))
		}

		// Codec selection
		var codec string
		switch p.VideoCodec {
		case "h264":
			if hwAccel == "nvenc" {
				codec = "h264_nvenc"
			} else if hwAccel == "vaapi" {
				codec = "h264_vaapi"
			} else if hwAccel == "qsv" {
				codec = "h264_qsv"
			} else {
				codec = "libx264"
			}
		case "hevc":
			if hwAccel == "nvenc" {
				codec = "hevc_nvenc"
			} else if hwAccel == "vaapi" {
				codec = "hevc_vaapi"
			} else if hwAccel == "qsv" {
				codec = "hevc_qsv"
			} else {
				codec = "libx265"
			}
		case "vp9":
			codec = "libvpx-vp9"
		default:
			codec = "libx264"
		}

		args = append(args, "-c:v", codec)

		// Bitrate
		if p.VideoBitrate > 0 {
			args = append(args, "-b:v", fmt.Sprintf("%dk", p.VideoBitrate))
		}

		// CRF for quality (only software)
		if hwAccel == "none" || hwAccel == "" || hwAccel == "auto" {
			if p.VideoCodec == "h264" {
				args = append(args, "-crf", "23", "-preset", "fast")
			} else if p.VideoCodec == "hevc" {
				args = append(args, "-crf", "28", "-preset", "fast")
			}
		}

		// Keyframe interval for HLS
		args = append(args, "-g", "48", "-keyint_min", "48")
	} else {
		// Audio only
		args = append(args, "-vn")
	}

	// Audio encoding
	if p.AudioCodec != "" {
		args = append(args, "-c:a", p.AudioCodec)
		if p.AudioBitrate > 0 {
			args = append(args, "-b:a", fmt.Sprintf("%dk", p.AudioBitrate))
		}
	} else {
		args = append(args, "-an")
	}

	// Container format
	if p.Container == "ts" {
		args = append(args, "-f", "mpegts")
	} else if p.Container == "mp4" {
		args = append(args, "-f", "mp4", "-movflags", "+faststart")
	}

	args = append(args, outputPath)
	return args
}

// BuildHLSCommand creates FFmpeg args for HLS segment generation
func BuildHLSCommand(inputPath, outputDir string, profile Profile, segmentDuration int, hwAccel string) []string {
	playlistPath := filepath.Join(outputDir, "playlist.m3u8")
	segmentPath := filepath.Join(outputDir, "segment_%03d.ts")

	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-i", inputPath,
		"-y",
	}

	// Video
	if profile.Width > 0 && profile.Height > 0 {
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d", profile.Width, profile.Height))
	}

	var codec string
	switch profile.VideoCodec {
	case "h264":
		codec = "libx264"
	case "hevc":
		codec = "libx265"
	default:
		codec = "libx264"
	}

	args = append(args,
		"-c:v", codec,
		"-crf", "23",
		"-preset", "fast",
		"-g", "48",
		"-keyint_min", "48",
		"-sc_threshold", "0",
	)

	if profile.VideoBitrate > 0 {
		args = append(args, "-b:v", fmt.Sprintf("%dk", profile.VideoBitrate))
	}

	// Audio
	args = append(args,
		"-c:a", "aac",
		"-b:a", fmt.Sprintf("%dk", profile.AudioBitrate),
		"-ac", "2",
	)

	// HLS output
	args = append(args,
		"-f", "hls",
		"-hls_time", strconv.Itoa(segmentDuration),
		"-hls_playlist_type", "vod",
		"-hls_segment_filename", segmentPath,
		"-hls_flags", "independent_segments",
		playlistPath,
	)

	return args
}

// DetectHWAccel probes for available hardware acceleration
func DetectHWAccel() string {
	// Try NVENC first
	cmd := exec.Command("ffmpeg", "-hide_banner", "-encoders")
	out, err := cmd.Output()
	if err != nil {
		return "none"
	}

	encoders := string(out)
	if strings.Contains(encoders, "h264_nvenc") {
		return "nvenc"
	}
	if strings.Contains(encoders, "h264_qsv") {
		return "qsv"
	}
	if strings.Contains(encoders, "h264_vaapi") {
		return "vaapi"
	}

	return "none"
}
