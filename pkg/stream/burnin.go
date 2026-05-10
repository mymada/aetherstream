package stream

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BurnInRequest configures subtitle burn-in parameters.
type BurnInRequest struct {
	ItemID       string `json:"item_id"`
	Language     string `json:"language"`
	OutputPath   string `json:"output_path,omitempty"`
	Profile      string `json:"profile,omitempty"`
	HWAccel      string `json:"hw_accel,omitempty"`
}

// BurnInResult returns the path to the burned-in output and metadata.
type BurnInResult struct {
	OutputPath string `json:"output_path"`
	Language   string `json:"language"`
	Profile    string `json:"profile"`
}

// ValidateBurnInRequest validates request fields and prevents path traversal.
func ValidateBurnInRequest(req BurnInRequest, mediaRoot string) error {
	if req.ItemID == "" {
		return fmt.Errorf("item_id required")
	}
	if req.Language == "" {
		return fmt.Errorf("language required")
	}
	if req.OutputPath != "" {
		cleanOut := filepath.Clean(req.OutputPath)
		cleanRoot := filepath.Clean(mediaRoot)
		if !strings.HasPrefix(cleanOut, cleanRoot+string(filepath.Separator)) && cleanOut != cleanRoot {
			return fmt.Errorf("invalid output_path")
		}
	}
	return nil
}

// BuildBurnInCommand builds an FFmpeg command that burns subtitles into the video.
// It uses the 'subtitles' video filter for text-based subs or 'overlay' for bitmap subs.
func BuildBurnInCommand(inputPath, subtitlePath, outputPath, profileName, hwAccel string) []string {
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-i", inputPath,
		"-y",
	}

	// Subtitle filter: use subtitles filter for SRT/ASS, fallback to overlay for bitmap
	subExt := strings.ToLower(filepath.Ext(subtitlePath))
	var vf string
	switch subExt {
	case ".srt", ".ass", ".ssa", ".vtt":
		vf = fmt.Sprintf("subtitles='%s'", subtitlePath)
	case ".sub", ".idx", ".sup":
		// Bitmap subs: overlay (simplified; real impl would need timing extraction)
		vf = fmt.Sprintf("overlay=0:0")
		args = append(args, "-i", subtitlePath)
	default:
		vf = fmt.Sprintf("subtitles='%s'", subtitlePath)
	}

	// Profile scaling
	p := getBurnInProfile(profileName)
	if p.Width > 0 && p.Height > 0 {
		vf = vf + fmt.Sprintf(",scale=%d:%d", p.Width, p.Height)
	}

	args = append(args, "-vf", vf)

	// Hardware acceleration (if requested and applicable)
	var codec string
	switch hwAccel {
	case "nvenc":
		codec = "h264_nvenc"
	case "qsv":
		codec = "h264_qsv"
	case "vaapi":
		codec = "h264_vaapi"
	default:
		codec = "libx264"
	}

	args = append(args,
		"-c:v", codec,
		"-crf", "23",
		"-preset", "fast",
		"-c:a", "copy",
		"-movflags", "+faststart",
	)

	if p.VideoBitrate > 0 {
		args = append(args, "-b:v", fmt.Sprintf("%dk", p.VideoBitrate))
	}

	args = append(args, outputPath)
	return args
}

type burnInProfile struct {
	Name         string
	Width        int
	Height       int
	VideoBitrate int
}

func getBurnInProfile(name string) burnInProfile {
	switch name {
	case "mobile_low":
		return burnInProfile{Name: "mobile_low", Width: 854, Height: 480, VideoBitrate: 800}
	case "mobile":
		return burnInProfile{Name: "mobile", Width: 1280, Height: 720, VideoBitrate: 2000}
	case "tablet":
		return burnInProfile{Name: "tablet", Width: 1920, Height: 1080, VideoBitrate: 4000}
	case "tv":
		return burnInProfile{Name: "tv", Width: 1920, Height: 1080, VideoBitrate: 6000}
	case "tv_4k":
		return burnInProfile{Name: "tv_4k", Width: 3840, Height: 2160, VideoBitrate: 15000}
	default:
		return burnInProfile{Name: "original", Width: 0, Height: 0, VideoBitrate: 0}
	}
}

// BurnIn performs subtitle burn-in for an item.
// It extracts the subtitle, runs FFmpeg, and returns the output path.
func BurnIn(itemPath, lang, mediaRoot, profileName, hwAccel string) (*BurnInResult, error) {
	// Extract subtitle to temp file
	subPath, err := extractSubtitleForBurnIn(itemPath, lang)
	if err != nil {
		return nil, fmt.Errorf("subtitle extraction: %w", err)
	}
	defer os.Remove(subPath)

	// Generate deterministic output path inside mediaRoot/transcodes/burnin
	hash := sha256.Sum256([]byte(itemPath + ":" + lang + ":" + profileName))
	outDir := filepath.Join(mediaRoot, "transcodes", "burnin")
	_ = os.MkdirAll(outDir, 0750)
	outPath := filepath.Join(outDir, fmt.Sprintf("%x.mp4", hash[:16]))

	args := BuildBurnInCommand(itemPath, subPath, outPath, profileName, hwAccel)
	// #nosec G204 - paths are validated before reaching here
	cmd := exec.Command("ffmpeg", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg burn-in failed: %w\n%s", err, output)
	}

	return &BurnInResult{
		OutputPath: outPath,
		Language:   lang,
		Profile:    profileName,
	}, nil
}

// extractSubtitleForBurnIn extracts the specified language subtitle to a temp SRT file.
func extractSubtitleForBurnIn(path, lang string) (string, error) {
	outPath := filepath.Join(os.TempDir(), "aetherstream_burnin_"+lang+".srt")
	// #nosec G204 - outPath is a fixed temp prefix; path validated by caller
	cmd := exec.Command("ffmpeg", "-i", path, "-map", "0:s:m:language:"+lang, outPath, "-y")
	if _, err := cmd.CombinedOutput(); err != nil {
		// Fallback: try first subtitle stream if language match fails
		// #nosec G204 - same validated parameters
		cmd2 := exec.Command("ffmpeg", "-i", path, "-map", "0:s:0", outPath, "-y")
		if _, err2 := cmd2.CombinedOutput(); err2 != nil {
			return "", fmt.Errorf("subtitle extraction failed: %w", err)
		}
	}
	return outPath, nil
}
