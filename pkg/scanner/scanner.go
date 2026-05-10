package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
)

// Supported media extensions
var videoExts = map[string]bool{
	".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
	".wmv": true, ".flv": true, ".webm": true, ".m4v": true,
	".ts": true, ".m2ts": true, ".mpeg": true, ".mpg": true,
}

var audioExts = map[string]bool{
	".mp3": true, ".flac": true, ".aac": true, ".ogg": true,
	".wma": true, ".m4a": true, ".wav": true, ".opus": true,
}

var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".bmp": true, ".tiff": true, ".webp": true,
}

// MediaFile represents a discovered media file
type MediaFile struct {
	Path       string
	Name       string
	Ext        string
	Size       int64
	ModTime    time.Time
	MediaType  string // video, audio, image
	LibraryID  string
}

// Scanner watches library paths and discovers media files
type Scanner struct {
	watcher   *fsnotify.Watcher
	libraries map[string]string // id -> path
	mu        sync.RWMutex
	results   chan MediaFile
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewScanner creates a filesystem scanner
func NewScanner() (*Scanner, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("fsnotify: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &Scanner{
		watcher:   w,
		libraries: make(map[string]string),
		results:   make(chan MediaFile, 100),
		ctx:       ctx,
		cancel:    cancel,
	}

	go s.watchLoop()
	return s, nil
}

// AddLibrary registers a library path for scanning
func (s *Scanner) AddLibrary(id, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.libraries[id] = path
	if err := s.watcher.Add(path); err != nil {
		return fmt.Errorf("watch %s: %w", path, err)
	}

	// Recursive add subdirectories
	return filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if d.IsDir() {
			return s.watcher.Add(p)
		}
		return nil
	})
}

// RemoveLibrary stops watching a library
func (s *Scanner) RemoveLibrary(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.libraries, id)
}

// ScanLibrary performs a full scan of a library path
func (s *Scanner) ScanLibrary(libraryID, path string) ([]MediaFile, error) {
	var files []MediaFile

	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}

		mf := s.classifyFile(p, d, libraryID)
		if mf != nil {
			files = append(files, *mf)
		}
		return nil
	})

	return files, err
}

// classifyFile determines media type from extension
func (s *Scanner) classifyFile(path string, d fs.DirEntry, libraryID string) *MediaFile {
	ext := strings.ToLower(filepath.Ext(path))

	var mediaType string
	switch {
	case videoExts[ext]:
		mediaType = "video"
	case audioExts[ext]:
		mediaType = "audio"
	case imageExts[ext]:
		mediaType = "image"
	default:
		return nil
	}

	info, err := d.Info()
	if err != nil {
		return nil
	}

	return &MediaFile{
		Path:      path,
		Name:      strings.TrimSuffix(filepath.Base(path), ext),
		Ext:       ext,
		Size:      info.Size(),
		ModTime:   info.ModTime(),
		MediaType: mediaType,
		LibraryID: libraryID,
	}
}

// watchLoop handles fsnotify events
func (s *Scanner) watchLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Create == fsnotify.Create {
				// New file or directory
				info, err := os.Stat(event.Name)
				if err != nil {
					continue
				}
				if info.IsDir() {
					s.watcher.Add(event.Name)
					continue
				}

				// Find which library this belongs to
				s.mu.RLock()
				var libID string
				for id, path := range s.libraries {
					if strings.HasPrefix(event.Name, path) {
						libID = id
						break
					}
				}
				s.mu.RUnlock()

				if libID != "" {
					mf := s.classifyFile(event.Name, &statDirEntry{info}, libID)
					if mf != nil {
						select {
						case s.results <- *mf:
						default:
						}
					}
				}
			}

		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			log.Error().Err(err).Msg("fsnotify error")
		}
	}
}

// Results returns the channel of discovered files
func (s *Scanner) Results() <-chan MediaFile {
	return s.results
}

// Close shuts down the scanner
func (s *Scanner) Close() error {
	s.cancel()
	return s.watcher.Close()
}

// statDirEntry wraps fs.FileInfo for fsnotify compatibility
type statDirEntry struct {
	info os.FileInfo
}

func (s *statDirEntry) Name() string       { return s.info.Name() }
func (s *statDirEntry) IsDir() bool        { return s.info.IsDir() }
func (s *statDirEntry) Type() fs.FileMode  { return s.info.Mode().Type() }
func (s *statDirEntry) Info() (os.FileInfo, error) { return s.info, nil }
