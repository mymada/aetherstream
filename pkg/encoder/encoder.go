package encoder

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Profile defines a transcoding preset
type Profile struct {
	Name         string
	Width        int
	Height       int
	VideoCodec   string // h264, hevc, vp9
	AudioCodec   string // aac, opus, ac3
	VideoBitrate int    // kbps
	AudioBitrate int    // kbps
	Container    string // mp4, ts, webm
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

// Hardware profiles (same resolutions, hardware codecs)
var (
	ProfileMobileLowNVENC = Profile{Name: "mobile_low_nvenc", Width: 854, Height: 480, VideoCodec: "h264", AudioCodec: "aac", VideoBitrate: 800, AudioBitrate: 128, Container: "ts"}
	ProfileMobileNVENC    = Profile{Name: "mobile_nvenc", Width: 1280, Height: 720, VideoCodec: "h264", AudioCodec: "aac", VideoBitrate: 2000, AudioBitrate: 128, Container: "ts"}
	ProfileTabletNVENC    = Profile{Name: "tablet_nvenc", Width: 1920, Height: 1080, VideoCodec: "h264", AudioCodec: "aac", VideoBitrate: 4000, AudioBitrate: 192, Container: "ts"}
	ProfileTVNVENC        = Profile{Name: "tv_nvenc", Width: 1920, Height: 1080, VideoCodec: "hevc", AudioCodec: "aac", VideoBitrate: 6000, AudioBitrate: 192, Container: "ts"}
	ProfileTV4KNVENC      = Profile{Name: "tv_4k_nvenc", Width: 3840, Height: 2160, VideoCodec: "hevc", AudioCodec: "aac", VideoBitrate: 15000, AudioBitrate: 192, Container: "ts"}

	ProfileMobileLowQSV = Profile{Name: "mobile_low_qsv", Width: 854, Height: 480, VideoCodec: "h264", AudioCodec: "aac", VideoBitrate: 800, AudioBitrate: 128, Container: "ts"}
	ProfileMobileQSV    = Profile{Name: "mobile_qsv", Width: 1280, Height: 720, VideoCodec: "h264", AudioCodec: "aac", VideoBitrate: 2000, AudioBitrate: 128, Container: "ts"}
	ProfileTabletQSV    = Profile{Name: "tablet_qsv", Width: 1920, Height: 1080, VideoCodec: "h264", AudioCodec: "aac", VideoBitrate: 4000, AudioBitrate: 192, Container: "ts"}
	ProfileTVQSV        = Profile{Name: "tv_qsv", Width: 1920, Height: 1080, VideoCodec: "hevc", AudioCodec: "aac", VideoBitrate: 6000, AudioBitrate: 192, Container: "ts"}
	ProfileTV4KQSV      = Profile{Name: "tv_4k_qsv", Width: 3840, Height: 2160, VideoCodec: "hevc", AudioCodec: "aac", VideoBitrate: 15000, AudioBitrate: 192, Container: "ts"}

	ProfileMobileLowVAAPI = Profile{Name: "mobile_low_vaapi", Width: 854, Height: 480, VideoCodec: "h264", AudioCodec: "aac", VideoBitrate: 800, AudioBitrate: 128, Container: "ts"}
	ProfileMobileVAAPI    = Profile{Name: "mobile_vaapi", Width: 1280, Height: 720, VideoCodec: "h264", AudioCodec: "aac", VideoBitrate: 2000, AudioBitrate: 128, Container: "ts"}
	ProfileTabletVAAPI    = Profile{Name: "tablet_vaapi", Width: 1920, Height: 1080, VideoCodec: "h264", AudioCodec: "aac", VideoBitrate: 4000, AudioBitrate: 192, Container: "ts"}
	ProfileTVVAAPI        = Profile{Name: "tv_vaapi", Width: 1920, Height: 1080, VideoCodec: "hevc", AudioCodec: "aac", VideoBitrate: 6000, AudioBitrate: 192, Container: "ts"}
	ProfileTV4KVAAPI      = Profile{Name: "tv_4k_vaapi", Width: 3840, Height: 2160, VideoCodec: "hevc", AudioCodec: "aac", VideoBitrate: 15000, AudioBitrate: 192, Container: "ts"}
)

// GetProfileByName returns a predefined profile (software or hardware)
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
	// NVENC profiles
	case "mobile_low_nvenc":
		return ProfileMobileLowNVENC
	case "mobile_nvenc":
		return ProfileMobileNVENC
	case "tablet_nvenc":
		return ProfileTabletNVENC
	case "tv_nvenc":
		return ProfileTVNVENC
	case "tv_4k_nvenc":
		return ProfileTV4KNVENC
	// QSV profiles
	case "mobile_low_qsv":
		return ProfileMobileLowQSV
	case "mobile_qsv":
		return ProfileMobileQSV
	case "tablet_qsv":
		return ProfileTabletQSV
	case "tv_qsv":
		return ProfileTVQSV
	case "tv_4k_qsv":
		return ProfileTV4KQSV
	// VAAPI profiles
	case "mobile_low_vaapi":
		return ProfileMobileLowVAAPI
	case "mobile_vaapi":
		return ProfileMobileVAAPI
	case "tablet_vaapi":
		return ProfileTabletVAAPI
	case "tv_vaapi":
		return ProfileTVVAAPI
	case "tv_4k_vaapi":
		return ProfileTV4KVAAPI
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

// BuildHLSCommand creates FFmpeg args for HLS segment generation.
// audioIndex selects which audio stream to include (0:a:N mapping).
func BuildHLSCommand(inputPath, outputDir string, profile Profile, segmentDuration int, hwAccel string, audioIndex int) []string {
	playlistPath := filepath.Join(outputDir, "playlist.m3u8")
	segmentPath := filepath.Join(outputDir, "segment_%03d.ts")

	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-i", inputPath,
		"-y",
		"-map", "0:v:0",
		"-map", fmt.Sprintf("0:a:%d", audioIndex),
	}

	// Hardware acceleration for HLS
	switch hwAccel {
	case "vaapi":
		args = append(args, "-hwaccel", "vaapi", "-vaapi_device", "/dev/dri/renderD128")
	case "nvenc":
		args = append(args, "-hwaccel", "cuda")
	case "qsv":
		args = append(args, "-hwaccel", "qsv")
	}

	// Video
	if profile.Width > 0 && profile.Height > 0 {
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d", profile.Width, profile.Height))
	}

	var codec string
	switch profile.VideoCodec {
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
	default:
		codec = "libx264"
	}

	args = append(args,
		"-c:v", codec,
		"-g", "48",
		"-keyint_min", "48",
		"-sc_threshold", "0",
	)

	// CRF only for software
	if hwAccel == "none" || hwAccel == "" || hwAccel == "auto" {
		args = append(args, "-crf", "23", "-preset", "fast")
	}

	if profile.VideoBitrate > 0 {
		args = append(args, "-b:v", fmt.Sprintf("%dk", profile.VideoBitrate))
	}

	// Audio
	args = append(args,
		"-c:a", "aac",
		"-b:a", fmt.Sprintf("%dk", profile.AudioBitrate),
		"-ac", "2",
	)

	// event playlist: ffmpeg writes playlist.m3u8 after each segment so clients can
	// start playback as soon as the first segment is ready, rather than waiting for
	// the full transcode. #EXT-X-ENDLIST is added when encoding completes.
	args = append(args,
		"-f", "hls",
		"-hls_time", strconv.Itoa(segmentDuration),
		"-hls_playlist_type", "event",
		"-hls_segment_filename", segmentPath,
		"-hls_flags", "independent_segments",
		playlistPath,
	)

	return args
}

// HardwareCapabilities holds detected GPU info
type HardwareCapabilities struct {
	NVENC  bool     `json:"nvenc"`
	QSV    bool     `json:"qsv"`
	VAAPI  bool     `json:"vaapi"`
	GPUs   []GPUInfo `json:"gpus"`
	Active string   `json:"active"`
}

// GPUInfo describes a detected GPU
type GPUInfo struct {
	Vendor  string `json:"vendor"`
	Model   string `json:"model"`
	Driver  string `json:"driver"`
	Backend string `json:"backend"` // nvenc, qsv, vaapi
}

var (
	hwCache     *HardwareCapabilities
	hwCacheOnce sync.Once
)

// DetectHardwareCapabilities probes for available GPUs and codecs
func DetectHardwareCapabilities() HardwareCapabilities {
	hwCacheOnce.Do(func() {
		hwCache = &HardwareCapabilities{
			GPUs: []GPUInfo{},
		}

		// 1) NVIDIA via nvidia-smi
		if gpus := detectNVIDIA(); len(gpus) > 0 {
			hwCache.NVENC = true
			hwCache.GPUs = append(hwCache.GPUs, gpus...)
		}

		// 2) Intel QuickSync via vainfo (Intel VAAPI/QuickSync detection)
		if gpus := detectIntel(); len(gpus) > 0 {
			hwCache.QSV = true
			hwCache.GPUs = append(hwCache.GPUs, gpus...)
		}

		// 3) AMD/Intel VAAPI via vainfo
		if gpus := detectVAAPI(); len(gpus) > 0 {
			hwCache.VAAPI = true
			hwCache.GPUs = append(hwCache.GPUs, gpus...)
		}

		// Fallback: also check ffmpeg encoders if no GPUs detected via tools
		if len(hwCache.GPUs) == 0 {
			detectFFmpegEncoders(hwCache)
		}

		// Determine active best backend (preference: nvenc > qsv > vaapi)
		switch {
		case hwCache.NVENC:
			hwCache.Active = "nvenc"
		case hwCache.QSV:
			hwCache.Active = "qsv"
		case hwCache.VAAPI:
			hwCache.Active = "vaapi"
		default:
			hwCache.Active = "none"
		}
	})
	return *hwCache
}

// detectNVIDIA probes NVIDIA GPUs via nvidia-smi
func detectNVIDIA() []GPUInfo {
	cmd := exec.Command("nvidia-smi", "--query-gpu=name,driver_version", "--format=csv,noheader")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var gpus []GPUInfo
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ",", 2)
		model := strings.TrimSpace(parts[0])
		driver := ""
		if len(parts) > 1 {
			driver = strings.TrimSpace(parts[1])
		}
		gpus = append(gpus, GPUInfo{
			Vendor:  "NVIDIA",
			Model:   model,
			Driver:  driver,
			Backend: "nvenc",
		})
	}
	return gpus
}

// detectIntel probes Intel GPUs via vainfo looking for Intel/iHD/i965
func detectIntel() []GPUInfo {
	cmd := exec.Command("vainfo")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}
	outStr := string(out)
	if !strings.Contains(outStr, "Intel") && !strings.Contains(outStr, "iHD") && !strings.Contains(outStr, "i965") {
		return nil
	}
	// Extract driver name from "libva version" / "vainfo: VA-API version" lines
	driver := "unknown"
	for _, line := range strings.Split(outStr, "\n") {
		if strings.Contains(line, "iHD") {
			driver = "iHD"
			break
		}
		if strings.Contains(line, "i965") {
			driver = "i965"
			break
		}
	}
	return []GPUInfo{{
		Vendor:  "Intel",
		Model:   "Intel GPU (QuickSync/VAAPI)",
		Driver:  driver,
		Backend: "qsv",
	}}
}

// detectVAAPI probes AMD/Intel GPUs via vainfo for VAAPI support
func detectVAAPI() []GPUInfo {
	cmd := exec.Command("vainfo")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}
	outStr := string(out)
	// Look for AMD/ATI or Mesa VAAPI
	if !strings.Contains(outStr, "AMD") && !strings.Contains(outStr, "ATI") && !strings.Contains(outStr, "Mesa") && !strings.Contains(outStr, "Radeon") {
		return nil
	}
	driver := "mesa"
	for _, line := range strings.Split(outStr, "\n") {
		if strings.Contains(line, "radeonsi") {
			driver = "radeonsi"
			break
		}
		if strings.Contains(line, " Mesa ") {
			driver = "mesa"
			break
		}
	}
	return []GPUInfo{{
		Vendor:  "AMD",
		Model:   "AMD GPU (VAAPI)",
		Driver:  driver,
		Backend: "vaapi",
	}}
}

// detectFFmpegEncoders checks ffmpeg -encoders as fallback
func detectFFmpegEncoders(hw *HardwareCapabilities) {
	cmd := exec.Command("ffmpeg", "-hide_banner", "-encoders")
	out, err := cmd.Output()
	if err != nil {
		return
	}
	encoders := string(out)
	if strings.Contains(encoders, "h264_nvenc") {
		hw.NVENC = true
	}
	if strings.Contains(encoders, "h264_qsv") {
		hw.QSV = true
	}
	if strings.Contains(encoders, "h264_vaapi") {
		hw.VAAPI = true
	}
}

// DetectHWAccel returns the best available hwaccel string (legacy compat)
func DetectHWAccel() string {
	caps := DetectHardwareCapabilities()
	return caps.Active
}

// SelectHWAccelWithFallback returns best hwaccel, falling back to software if unavailable
func SelectHWAccelWithFallback(preferred string) string {
	caps := DetectHardwareCapabilities()
	if preferred != "" && preferred != "auto" && preferred != "none" {
		switch preferred {
		case "nvenc":
			if caps.NVENC {
				return "nvenc"
			}
		case "qsv":
			if caps.QSV {
				return "qsv"
			}
		case "vaapi":
			if caps.VAAPI {
				return "vaapi"
			}
		}
	}
	return caps.Active
}
