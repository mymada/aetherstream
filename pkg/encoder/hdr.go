package encoder

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// HDRInfo describes HDR metadata detected in a media file
type HDRInfo struct {
	IsHDR            bool     `json:"is_hdr"`
	HDRFormat        string   `json:"hdr_format,omitempty"`
	ColorPrimaries   string   `json:"color_primaries,omitempty"`
	ColorTransfer    string   `json:"color_transfer,omitempty"`
	ColorSpace       string   `json:"color_space,omitempty"`
	MaxCLL           int      `json:"max_cll,omitempty"`    // Max Content Light Level (nits)
	MaxFALL          int      `json:"max_fall,omitempty"`   // Max Frame Average Light Level
	MasteringDisplay string   `json:"mastering_display,omitempty"`
	SideData         []string `json:"side_data,omitempty"`
}

// ToneMappingMode defines the tone mapping algorithm to use
type ToneMappingMode string

const (
	ToneMapHable     ToneMappingMode = "hable"
	ToneMapReinhard  ToneMappingMode = "reinhard"
	ToneMapMobius    ToneMappingMode = "mobius"
	ToneMapClip      ToneMappingMode = "clip"
	ToneMapBT2390    ToneMappingMode = "bt.2390"
)

// ToneMappingOptions configures HDR to SDR tone mapping
type ToneMappingOptions struct {
	Mode        ToneMappingMode
	Desat       float64 // 0.0 - 1.0, desaturation strength
	PeakNits    int     // target display peak brightness (default 100)
	SourceNits  int     // source content peak brightness
}

// DefaultToneMapping returns sensible defaults for SDR displays
func DefaultToneMapping() ToneMappingOptions {
	return ToneMappingOptions{
		Mode:     ToneMapHable,
		Desat:    0.5,
		PeakNits: 100,
	}
}

// DetectHDR probes a media file for HDR metadata using ffprobe
func DetectHDR(filePath string) (*HDRInfo, error) {
	// #nosec G204 - ffprobe with fixed arguments, filePath validated by caller
	cmd := exec.Command("ffprobe",
		"-hide_banner",
		"-loglevel", "error",
		"-select_streams", "v:0",
		"-show_streams",
		"-show_frames",
		"-read_intervals", "%+#1",
		"-show_entries", "frame=side_data_list",
		"-of", "default=noprint_wrappers=1",
		filePath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffprobe hdr detection: %w", err)
	}
	return parseHDRInfo(string(out)), nil
}

// parseHDRInfo extracts HDR fields from ffprobe output
func parseHDRInfo(raw string) *HDRInfo {
	info := &HDRInfo{}
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "color_primaries=") {
			info.ColorPrimaries = extractValue(line, "color_primaries=")
			if info.ColorPrimaries == "bt2020" {
				info.IsHDR = true
			}
		}
		if strings.Contains(line, "color_transfer=") {
			info.ColorTransfer = extractValue(line, "color_transfer=")
			if strings.Contains(info.ColorTransfer, "smpte2084") || strings.Contains(info.ColorTransfer, "arib-std-b67") {
				info.IsHDR = true
			}
		}
		if strings.Contains(line, "colorspace=") || strings.Contains(line, "color_space=") {
			info.ColorSpace = extractValue(line, "colorspace=")
			if info.ColorSpace == "" {
				info.ColorSpace = extractValue(line, "color_space=")
			}
		}
		if strings.Contains(line, "HDR_FORMAT=") || strings.Contains(line, "hdr_format=") {
			info.HDRFormat = extractValue(line, "HDR_FORMAT=")
			if info.HDRFormat == "" {
				info.HDRFormat = extractValue(line, "hdr_format=")
			}
		}
		if strings.Contains(line, "max_cll=") {
			v := extractValue(line, "max_cll=")
			info.MaxCLL, _ = strconv.Atoi(v)
		}
		if strings.Contains(line, "max_fall=") {
			v := extractValue(line, "max_fall=")
			info.MaxFALL, _ = strconv.Atoi(v)
		}
		if strings.Contains(line, "Mastering display") || strings.Contains(line, "mastering_display=") {
			info.MasteringDisplay = extractValue(line, "mastering_display=")
		}
		if strings.Contains(line, "side_data_type=") {
			info.SideData = append(info.SideData, extractValue(line, "side_data_type="))
		}
	}
	if info.HDRFormat == "" && info.IsHDR {
		if strings.Contains(info.ColorTransfer, "smpte2084") {
			info.HDRFormat = "HDR10"
		} else if strings.Contains(info.ColorTransfer, "arib-std-b67") {
			info.HDRFormat = "HLG"
		}
	}
	return info
}

func extractValue(line, key string) string {
	idx := strings.Index(line, key)
	if idx == -1 {
		return ""
	}
	val := line[idx+len(key):]
	val = strings.TrimSpace(val)
	return strings.Trim(val, "\"")
}

// BuildToneMappingFilter creates an FFmpeg filter string for HDR to SDR conversion
func BuildToneMappingFilter(opts ToneMappingOptions) string {
	mode := string(opts.Mode)
	if mode == "" {
		mode = string(ToneMapHable)
	}
	peak := opts.PeakNits
	if peak <= 0 {
		peak = 100
	}
	desat := opts.Desat
	if desat < 0 {
		desat = 0
	} else if desat > 1 {
		desat = 1
	}
	return fmt.Sprintf("zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=%s:desat=%.2f:peak=%d,zscale=t=bt709:m=bt709:r=tv", mode, desat, peak)
}

// ApplyToneMappingToArgs injects tone mapping filters into existing FFmpeg args
func ApplyToneMappingToArgs(args []string, opts ToneMappingOptions) []string {
	filter := BuildToneMappingFilter(opts)
	// Insert after the first -vf if present, otherwise add -vf
	vfIdx := -1
	for i, a := range args {
		if a == "-vf" {
			vfIdx = i
			break
		}
	}
	if vfIdx >= 0 && vfIdx+1 < len(args) {
		existing := args[vfIdx+1]
		args[vfIdx+1] = existing + "," + filter
	} else {
		// Find position after input (-i) to insert -vf
		insertIdx := len(args)
		for i, a := range args {
			if a == "-i" {
				insertIdx = i + 2
				break
			}
		}
		before := append([]string{}, args[:insertIdx]...)
		after := append([]string{}, args[insertIdx:]...)
		args = append(before, "-vf", filter)
		args = append(args, after...)
	}
	return args
}
