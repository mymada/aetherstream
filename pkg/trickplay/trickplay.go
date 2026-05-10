package trickplay

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Service generates trickplay / chapter preview images at regular intervals.
type Service struct {
	ffmpegPath string
	outputDir  string
	interval   time.Duration // interval between preview images
	width      int           // max width of preview images
}

// NewService creates a trickplay generator.
func NewService(ffmpegPath, outputDir string) *Service {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	return &Service{
		ffmpegPath: ffmpegPath,
		outputDir:  outputDir,
		interval:   10 * time.Second,
		width:      320,
	}
}

// SetInterval changes the interval between preview frames.
func (s *Service) SetInterval(d time.Duration) {
	s.interval = d
}

// SetWidth changes the output width (height auto-scales).
func (s *Service) SetWidth(w int) {
	s.width = w
}

// Generate creates preview images for a video at the configured interval.
// Images are written as {itemID}_thumb_%04d.jpg in the output directory.
func (s *Service) Generate(videoPath, itemID string, durationSeconds float64) ([]string, error) {
	if err := os.MkdirAll(s.outputDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	outPattern := filepath.Join(s.outputDir, fmt.Sprintf("%s_thumb_%%04d.jpg", itemID))
	fps := 1.0 / s.interval.Seconds()
	fpsStr := strconv.FormatFloat(fps, 'f', 4, 64)
	scale := fmt.Sprintf("scale=%d:-1", s.width)

	// #nosec G204 - videoPath validated by caller against library paths
	cmd := exec.Command(s.ffmpegPath,
		"-hide_banner",
		"-loglevel", "error",
		"-i", videoPath,
		"-vf", fmt.Sprintf("fps=%s,%s", fpsStr, scale),
		"-q:v", "4",
		"-y",
		outPattern,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg trickplay failed: %w: %s", err, string(out))
	}

	// Collect generated files
	files, err := os.ReadDir(s.outputDir)
	if err != nil {
		return nil, err
	}
	var thumbs []string
	prefix := itemID + "_thumb_"
	for _, f := range files {
		if !f.IsDir() && strings.HasPrefix(f.Name(), prefix) && strings.HasSuffix(f.Name(), ".jpg") {
			thumbs = append(thumbs, filepath.Join(s.outputDir, f.Name()))
		}
	}
	return thumbs, nil
}

// Exists checks if any trickplay images exist for an item.
func (s *Service) Exists(itemID string) bool {
	files, err := os.ReadDir(s.outputDir)
	if err != nil {
		return false
	}
	prefix := itemID + "_thumb_"
	for _, f := range files {
		if !f.IsDir() && strings.HasPrefix(f.Name(), prefix) {
			return true
		}
	}
	return false
}

// List returns the paths of all preview images for an item.
func (s *Service) List(itemID string) []string {
	files, err := os.ReadDir(s.outputDir)
	if err != nil {
		return nil
	}
	var thumbs []string
	prefix := itemID + "_thumb_"
	for _, f := range files {
		if !f.IsDir() && strings.HasPrefix(f.Name(), prefix) && strings.HasSuffix(f.Name(), ".jpg") {
			thumbs = append(thumbs, filepath.Join(s.outputDir, f.Name()))
		}
	}
	return thumbs
}
