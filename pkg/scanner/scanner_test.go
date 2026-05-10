package scanner

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClassifyFileVideo(t *testing.T) {
	s, err := NewScanner()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "movie.mp4")
	if err := os.WriteFile(path, []byte("dummy"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	mf := s.classifyFile(path, &statDirEntry{info}, "lib1")
	if mf == nil {
		t.Fatal("expected media file")
	}
	if mf.MediaType != "video" {
		t.Errorf("MediaType = %q, want video", mf.MediaType)
	}
	if mf.Name != "movie" {
		t.Errorf("Name = %q, want movie", mf.Name)
	}
	if mf.Ext != ".mp4" {
		t.Errorf("Ext = %q, want .mp4", mf.Ext)
	}
	if mf.LibraryID != "lib1" {
		t.Errorf("LibraryID = %q, want lib1", mf.LibraryID)
	}
}

func TestClassifyFileAudio(t *testing.T) {
	s, err := NewScanner()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "song.flac")
	if err := os.WriteFile(path, []byte("dummy"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	mf := s.classifyFile(path, &statDirEntry{info}, "lib2")
	if mf == nil {
		t.Fatal("expected media file")
	}
	if mf.MediaType != "audio" {
		t.Errorf("MediaType = %q, want audio", mf.MediaType)
	}
}

func TestClassifyFileImage(t *testing.T) {
	s, err := NewScanner()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "photo.jpg")
	if err := os.WriteFile(path, []byte("dummy"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	mf := s.classifyFile(path, &statDirEntry{info}, "lib3")
	if mf == nil {
		t.Fatal("expected media file")
	}
	if mf.MediaType != "image" {
		t.Errorf("MediaType = %q, want image", mf.MediaType)
	}
}

func TestClassifyFileUnsupported(t *testing.T) {
	s, err := NewScanner()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "readme.txt")
	if err := os.WriteFile(path, []byte("dummy"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	mf := s.classifyFile(path, &statDirEntry{info}, "lib1")
	if mf != nil {
		t.Error("expected nil for unsupported extension")
	}
}

func TestScanLibrary(t *testing.T) {
	s, err := NewScanner()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	dir := t.TempDir()
	files := []string{"a.mp4", "b.mkv", "c.mp3", "d.txt"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	results, err := s.ScanLibrary("lib1", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 media files, got %d", len(results))
	}

	var hasVideo, hasAudio bool
	for _, mf := range results {
		if mf.MediaType == "video" {
			hasVideo = true
		}
		if mf.MediaType == "audio" {
			hasAudio = true
		}
		if mf.LibraryID != "lib1" {
			t.Errorf("LibraryID = %q, want lib1", mf.LibraryID)
		}
	}
	if !hasVideo {
		t.Error("expected video files")
	}
	if !hasAudio {
		t.Error("expected audio files")
	}
}

func TestScannerAddRemoveLibrary(t *testing.T) {
	s, err := NewScanner()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	dir := t.TempDir()
	if err := s.AddLibrary("movies", dir); err != nil {
		t.Fatal(err)
	}

	s.mu.RLock()
	path, ok := s.libraries["movies"]
	s.mu.RUnlock()
	if !ok {
		t.Fatal("library not registered")
	}
	if path != dir {
		t.Errorf("path = %q, want %q", path, dir)
	}

	s.RemoveLibrary("movies")
	s.mu.RLock()
	_, ok = s.libraries["movies"]
	s.mu.RUnlock()
	if ok {
		t.Error("library should have been removed")
	}
}

func TestStatDirEntry(t *testing.T) {
	info, err := os.Stat(".")
	if err != nil {
		t.Fatal(err)
	}
	entry := &statDirEntry{info}
	if entry.Name() != info.Name() {
		t.Error("Name mismatch")
	}
	if entry.IsDir() != info.IsDir() {
		t.Error("IsDir mismatch")
	}
	if entry.Type() != info.Mode().Type() {
		t.Error("Type mismatch")
	}
	gotInfo, err := entry.Info()
	if err != nil {
		t.Fatal(err)
	}
	if gotInfo != info {
		t.Error("Info mismatch")
	}
}

func TestResultsChannel(t *testing.T) {
	s, err := NewScanner()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ch := s.Results()
	if ch == nil {
		t.Error("Results channel is nil")
	}
}

func TestScannerClose(t *testing.T) {
	s, err := NewScanner()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	// Ensure context is cancelled
	select {
	case <-s.ctx.Done():
		// ok
	case <-time.After(time.Second):
		t.Error("context not cancelled after Close")
	}
}
