package thumbnail

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestService_BackdropPath(t *testing.T) {
	s := NewService("ffmpeg", "/tmp/thumbs")
	path := s.Path("item-abc", TypeBackdrop)
	expected := filepath.Join("/tmp/thumbs", "item-abc_backdrop.jpg")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestService_DefaultOffsets(t *testing.T) {
	s := NewService("ffmpeg", "/tmp/thumbs")
	if s.posterOffset != 10*time.Second {
		t.Errorf("expected posterOffset 10s, got %v", s.posterOffset)
	}
	if s.backdropOffset != 30*time.Second {
		t.Errorf("expected backdropOffset 30s, got %v", s.backdropOffset)
	}
}

func TestService_PosterAndBackdropPathDistinct(t *testing.T) {
	s := NewService("ffmpeg", "/tmp/thumbs")
	poster := s.Path("item-1", TypePoster)
	backdrop := s.Path("item-1", TypeBackdrop)
	if poster == backdrop {
		t.Error("poster and backdrop paths should be distinct")
	}
}

func TestGenerateThumbnails_BothExist_SkipsFFmpeg(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewService("ffmpeg", tmpDir)

	posterPath := s.Path("item-999", TypePoster)
	backdropPath := s.Path("item-999", TypeBackdrop)
	if err := os.WriteFile(posterPath, []byte("poster"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backdropPath, []byte("backdrop"), 0644); err != nil {
		t.Fatal(err)
	}

	pp, bp, err := s.GenerateThumbnails("/fake/video.mp4", "item-999")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if pp != posterPath {
		t.Errorf("expected poster %s, got %s", posterPath, pp)
	}
	if bp != backdropPath {
		t.Errorf("expected backdrop %s, got %s", backdropPath, bp)
	}
}

func TestService_Exists_Poster(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewService("ffmpeg", tmpDir)

	assert_false := func(v bool, msg string) {
		t.Helper()
		if v {
			t.Error(msg)
		}
	}
	assert_true := func(v bool, msg string) {
		t.Helper()
		if !v {
			t.Error(msg)
		}
	}

	assert_false(s.Exists("item-x", TypePoster), "should not exist before creation")

	path := s.Path("item-x", TypePoster)
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	assert_true(s.Exists("item-x", TypePoster), "should exist after creation")
	assert_false(s.Exists("item-x", TypeBackdrop), "backdrop should not exist")
}

func TestService_extract_SkipsExisting_Backdrop(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewService("nonexistent-ffmpeg", tmpDir)

	path := s.Path("item-backdrop", TypeBackdrop)
	if err := os.WriteFile(path, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := s.extract("/fake/video.mp4", "item-backdrop", TypeBackdrop, s.backdropOffset)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != path {
		t.Errorf("expected %s, got %s", path, result)
	}
}

func TestService_extract_MkdirAll(t *testing.T) {
	tmpBase := t.TempDir()
	subDir := filepath.Join(tmpBase, "nested", "deep", "thumbs")
	s := NewService("ffmpeg", subDir)

	// Create the file inside the (not-yet-created) nested dir
	if err := os.MkdirAll(subDir, 0750); err != nil {
		t.Fatal(err)
	}
	existing := s.Path("item-nested", TypePoster)
	if err := os.WriteFile(existing, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := s.extract("/any/path.mp4", "item-nested", TypePoster, s.posterOffset)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != existing {
		t.Errorf("expected %s, got %s", existing, result)
	}
}

func TestExtractPoster_SkipsExisting(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewService("nonexistent-ffmpeg", tmpDir)

	path := s.Path("item-ep", TypePoster)
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := s.ExtractPoster("/fake/video.mp4", "item-ep")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != path {
		t.Errorf("expected %s, got %s", path, result)
	}
}

func TestExtractBackdrop_SkipsExisting(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewService("nonexistent-ffmpeg", tmpDir)

	path := s.Path("item-eb", TypeBackdrop)
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := s.ExtractBackdrop("/fake/video.mp4", "item-eb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != path {
		t.Errorf("expected %s, got %s", path, result)
	}
}
