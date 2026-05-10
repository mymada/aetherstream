package thumbnail

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// ThumbnailType represents the kind of thumbnail to extract
type ThumbnailType string

const (
	TypePoster    ThumbnailType = "poster"
	TypeBackdrop  ThumbnailType = "backdrop"
)

// Service handles thumbnail generation via FFmpeg
type Service struct {
	ffmpegPath    string
	outputDir     string
	posterOffset  time.Duration
	backdropOffset time.Duration
}

// NewService creates a thumbnail generator
func NewService(ffmpegPath, outputDir string) *Service {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	return &Service{
		ffmpegPath:     ffmpegPath,
		outputDir:      outputDir,
		posterOffset:   10 * time.Second,
		backdropOffset: 30 * time.Second,
	}
}

// ExtractPoster generates a poster thumbnail at the default poster offset
func (s *Service) ExtractPoster(videoPath, itemID string) (string, error) {
	return s.extract(videoPath, itemID, TypePoster, s.posterOffset)
}

// ExtractBackdrop generates a backdrop thumbnail at the default backdrop offset
func (s *Service) ExtractBackdrop(videoPath, itemID string) (string, error) {
	return s.extract(videoPath, itemID, TypeBackdrop, s.backdropOffset)
}

// GenerateThumbnails creates both poster and backdrop for a video
func (s *Service) GenerateThumbnails(videoPath, itemID string) (posterPath, backdropPath string, err error) {
	posterPath, err = s.ExtractPoster(videoPath, itemID)
	if err != nil {
		return "", "", fmt.Errorf("poster: %w", err)
	}
	backdropPath, err = s.ExtractBackdrop(videoPath, itemID)
	if err != nil {
		return posterPath, "", fmt.Errorf("backdrop: %w", err)
	}
	return posterPath, backdropPath, nil
}

// Path returns the on-disk path for a given item and thumbnail type
func (s *Service) Path(itemID string, t ThumbnailType) string {
	return filepath.Join(s.outputDir, fmt.Sprintf("%s_%s.jpg", itemID, t))
}

// Exists checks if a thumbnail already exists on disk
func (s *Service) Exists(itemID string, t ThumbnailType) bool {
	_, err := os.Stat(s.Path(itemID, t))
	return err == nil
}

// extract runs FFmpeg to grab a single frame at the given offset
func (s *Service) extract(videoPath, itemID string, t ThumbnailType, offset time.Duration) (string, error) {
	outPath := s.Path(itemID, t)

	if err := os.MkdirAll(s.outputDir, 0750); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	// Avoid overwriting if already present (callers can check Exists first)
	if _, err := os.Stat(outPath); err == nil {
		return outPath, nil
	}

	sec := strconv.FormatFloat(offset.Seconds(), 'f', 3, 64)
	// #nosec G204 - videoPath is validated by caller against library paths
	cmd := exec.Command(s.ffmpegPath,
		"-hide_banner",
		"-loglevel", "error",
		"-ss", sec,
		"-i", videoPath,
		"-vframes", "1",
		"-q:v", "2",
		"-y",
		outPath,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg failed: %w: %s", err, string(out))
	}

	return outPath, nil
}
