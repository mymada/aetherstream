package encoder

import (
	"fmt"
	"os/exec"
	"strings"
)

// DolbyVisionProfile describes Dolby Vision profile metadata
type DolbyVisionProfile struct {
	Profile       int    `json:"profile"`
	Level         string `json:"level,omitempty"`
	RPUPresent    bool   `json:"rpu_present"`
	BLPresent     bool   `json:"bl_present"` // Base layer present
	ELPresent     bool   `json:"el_present"` // Enhancement layer present
	Compatibility string `json:"compatibility,omitempty"` // e.g. "BL+EL+RPU", "BL+RPU"
	Codec         string `json:"codec,omitempty"`
}

// DolbyVisionInfo holds detection results for Dolby Vision content
type DolbyVisionInfo struct {
	IsDolbyVision bool               `json:"is_dolby_vision"`
	Profile       DolbyVisionProfile `json:"profile"`
	HDRInfo       *HDRInfo           `json:"hdr_info,omitempty"`
}

// DetectDolbyVision probes a media file for Dolby Vision metadata using ffprobe
func DetectDolbyVision(filePath string) (*DolbyVisionInfo, error) {
	// #nosec G204 - ffprobe with fixed arguments, filePath validated by caller
	cmd := exec.Command("ffprobe",
		"-hide_banner",
		"-loglevel", "error",
		"-select_streams", "v:0",
		"-show_streams",
		"-show_frames",
		"-read_intervals", "%+#1",
		"-show_entries", "stream=codec_name:frame=side_data_list",
		"-of", "default=noprint_wrappers=1",
		filePath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffprobe dolby vision detection: %w", err)
	}
	return parseDolbyVisionInfo(string(out)), nil
}

func parseDolbyVisionInfo(raw string) *DolbyVisionInfo {
	info := &DolbyVisionInfo{
		Profile: DolbyVisionProfile{},
	}
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "codec_name=") {
			info.Profile.Codec = extractValue(line, "codec_name=")
		}
		if strings.Contains(line, "Dolby Vision") || strings.Contains(line, "dolby vision") {
			info.IsDolbyVision = true
		}
		if strings.Contains(line, "profile=") {
			v := extractValue(line, "profile=")
			if v != "" {
				var p int
				_, _ = fmt.Sscanf(v, "%d", &p)
				info.Profile.Profile = p
			}
		}
		if strings.Contains(line, "level=") {
			info.Profile.Level = extractValue(line, "level=")
		}
		if strings.Contains(line, "rpu_present=") {
			info.Profile.RPUPresent = extractValue(line, "rpu_present=") == "1" || extractValue(line, "rpu_present=") == "true"
		}
		if strings.Contains(line, "bl_present=") {
			info.Profile.BLPresent = extractValue(line, "bl_present=") == "1" || extractValue(line, "bl_present=") == "true"
		}
		if strings.Contains(line, "el_present=") {
			info.Profile.ELPresent = extractValue(line, "el_present=") == "1" || extractValue(line, "el_present=") == "true"
		}
		if strings.Contains(line, "compatibility=") {
			info.Profile.Compatibility = extractValue(line, "compatibility=")
		}
	}
	if info.IsDolbyVision {
		if info.Profile.Compatibility == "" {
			if info.Profile.ELPresent {
				info.Profile.Compatibility = "BL+EL+RPU"
			} else if info.Profile.RPUPresent {
				info.Profile.Compatibility = "BL+RPU"
			} else {
				info.Profile.Compatibility = "BL"
			}
		}
	}
	return info
}

// DolbyVisionTranscodeOptions configures Dolby Vision aware transcoding
type DolbyVisionTranscodeOptions struct {
	PreserveDolbyVision bool               // pass through Dolby Vision if client supports it
	FallbackToHDR10     bool               // fallback to HDR10 if DV not supported
	FallbackToSDR       bool               // tone map to SDR if neither DV nor HDR10 supported
	ToneMapOpts         ToneMappingOptions
}

// DefaultDolbyVisionOptions returns safe defaults for wide compatibility
func DefaultDolbyVisionOptions() DolbyVisionTranscodeOptions {
	return DolbyVisionTranscodeOptions{
		PreserveDolbyVision: false,
		FallbackToHDR10:     true,
		FallbackToSDR:       true,
		ToneMapOpts:         DefaultToneMapping(),
	}
}

// BuildDolbyVisionFilter creates FFmpeg filter for Dolby Vision handling
func BuildDolbyVisionFilter(dvInfo *DolbyVisionInfo, opts DolbyVisionTranscodeOptions) string {
	if dvInfo == nil || !dvInfo.IsDolbyVision {
		return ""
	}
	if opts.PreserveDolbyVision {
		return "copy"
	}
	if opts.FallbackToHDR10 {
		return "dolbyvision=dovi=1:el=0"
	}
	if opts.FallbackToSDR {
		return BuildToneMappingFilter(opts.ToneMapOpts)
	}
	return ""
}

// DolbyVisionSupportedProfiles returns profiles commonly supported by clients
func DolbyVisionSupportedProfiles() []int {
	return []int{5, 8}
}

// IsProfileSupported checks if a Dolby Vision profile is in the supported list
func IsProfileSupported(profile int) bool {
	for _, p := range DolbyVisionSupportedProfiles() {
		if p == profile {
			return true
		}
	}
	return false
}
