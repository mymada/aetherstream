package thumbnail

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewService(t *testing.T) {
	s := NewService("/usr/bin/ffmpeg", "./thumbs")
	if s.ffmpegPath != "/usr/bin/ffmpeg" {
		t.Errorf("expected ffmpegPath /usr/bin/ffmpeg, got %s", s.ffmpegPath)
	}
	if s.outputDir != "./thumbs" {
		t.Errorf("expected outputDir ./thumbs, got %s", s.outputDir)
	}
}

func TestNewService_DefaultPath(t *testing.T) {
	s := NewService("", "./thumbs")
	if s.ffmpegPath != "ffmpeg" {
		t.Errorf("expected default ffmpegPath 'ffmpeg', got %s", s.ffmpegPath)
	}
}

func TestService_Path(t *testing.T) {
	s := NewService("ffmpeg", "/tmp/thumbs")
	path := s.Path("item-123", TypePoster)
	expected := filepath.Join("/tmp/thumbs", "item-123_poster.jpg")
	if path != expected {
		t.Errorf("expected path %s, got %s", expected, path)
	}
}

func TestService_Exists_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewService("ffmpeg", tmpDir)
	exists := s.Exists("nonexistent", TypePoster)
	if exists {
		t.Error("expected Exists to return false for non-existent file")
	}
}

func TestService_Exists_Found(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewService("ffmpeg", tmpDir)
	
	// Create a dummy file
	path := s.Path("item-123", TypeBackdrop)
	if err := os.WriteFile(path, []byte("dummy"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	
	exists := s.Exists("item-123", TypeBackdrop)
	if !exists {
		t.Error("expected Exists to return true for existing file")
	}
}

func TestService_extract_SkipsExisting(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewService("ffmpeg", tmpDir)
	
	// Pre-create the output file
	path := s.Path("item-456", TypePoster)
	if err := os.WriteFile(path, []byte("existing"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	
	// Should return existing path without running ffmpeg
	result, err := s.extract("/fake/video.mp4", "item-456", TypePoster, s.posterOffset)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != path {
		t.Errorf("expected path %s, got %s", path, result)
	}
}
