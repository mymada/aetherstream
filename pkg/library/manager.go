package library

import (
	"fmt"
	"time"

	"github.com/devuser/aetherstream/pkg/cache"
	"github.com/devuser/aetherstream/pkg/db"
	"github.com/devuser/aetherstream/pkg/metadata"
	"github.com/devuser/aetherstream/pkg/naming"
	"github.com/devuser/aetherstream/pkg/scanner"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Manager handles library operations: scan, metadata, CRUD
type Manager struct {
	db        *db.DB
	scanner   *scanner.Scanner
	tmdb      *metadata.TMDbClient
	scanQueue chan scanJob
	cache     cache.Cache
}

type scanJob struct {
	libraryID string
	path      string
}

// NewManager creates a library manager
func NewManager(database *db.DB, tmdbKey string) (*Manager, error) {
	s, err := scanner.NewScanner()
	if err != nil {
		return nil, fmt.Errorf("scanner: %w", err)
	}

	m := &Manager{
		db:        database,
		scanner:   s,
		tmdb:      metadata.NewTMDbClient(tmdbKey),
		scanQueue: make(chan scanJob, 10),
		cache:     cache.NewLRUCache(1000),
	}

	go m.scanWorker()
	go m.watchWorker()

	return m, nil
}

// CreateLibrary adds a new library and starts scanning
func (m *Manager) CreateLibrary(name, path, mediaType string) (string, error) {
	id := uuid.New().String()
	if err := m.db.CreateLibrary(id, name, path, mediaType); err != nil {
		return "", fmt.Errorf("create library: %w", err)
	}

	if err := m.scanner.AddLibrary(id, path); err != nil {
		log.Warn().Err(err).Str("path", path).Msg("failed to watch library path")
	}

	// Queue initial scan
	select {
	case m.scanQueue <- scanJob{libraryID: id, path: path}:
	default:
	}

	return id, nil
}

// ScanLibrary triggers a manual full scan
func (m *Manager) ScanLibrary(libraryID string) error {
	libs, err := m.db.ListLibraries()
	if err != nil {
		return err
	}

	for _, lib := range libs {
		if lib["id"] == libraryID {
			path, _ := lib["path"].(string)
			select {
			case m.scanQueue <- scanJob{libraryID: libraryID, path: path}:
			default:
				return fmt.Errorf("scan queue full")
			}
			return nil
		}
	}

	return fmt.Errorf("library not found: %s", libraryID)
}

// scanWorker processes scan jobs
func (m *Manager) scanWorker() {
	for job := range m.scanQueue {
		log.Info().Str("library", job.libraryID).Str("path", job.path).Msg("scanning library")

		files, err := m.scanner.ScanLibrary(job.libraryID, job.path)
		if err != nil {
			log.Error().Err(err).Str("library", job.libraryID).Msg("scan failed")
			continue
		}

		for _, f := range files {
			if err := m.processFile(f); err != nil {
				log.Warn().Err(err).Str("file", f.Path).Msg("process file failed")
			}
		}

		log.Info().Str("library", job.libraryID).Int("files", len(files)).Msg("scan complete")
	}
}

// watchWorker handles real-time file discovery
func (m *Manager) watchWorker() {
	for f := range m.scanner.Results() {
		if err := m.processFile(f); err != nil {
			log.Warn().Err(err).Str("file", f.Path).Msg("watch process failed")
		}
	}
}

// processFile classifies, parses naming, fetches metadata, stores in DB
func (m *Manager) processFile(f scanner.MediaFile) error {
	// Parse filename for metadata
	parsed := naming.ParseFilename(f.Path)

	// Generate item ID
	itemID := uuid.New().String()

	// Fetch metadata for movies
	if parsed.Kind == "movie" && m.tmdb != nil {
		if result, err := m.tmdb.SearchMovie(parsed.Title, parsed.Year); err == nil && result != nil {
			details, _ := m.tmdb.GetMovieDetails(result.ID)
			if details != nil && details.PosterPath != "" {
				posterURL := metadata.PosterURL(details.PosterPath, "w500")
				m.cache.Set(cache.PosterKey(itemID), posterURL, 24*time.Hour)
			}
		}
	}

	// Store in database
	return m.db.CreateItem(
		itemID,
		f.LibraryID,
		f.Path,
		f.Name,
		parsed.Kind,
		f.Ext,
		f.Size,
		0, // duration — will be filled by probe
		0, 0,
		"", "",
	)
}

// Close shuts down the manager
func (m *Manager) Close() error {
	close(m.scanQueue)
	return m.scanner.Close()
}
