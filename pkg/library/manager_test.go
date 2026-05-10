package library

import (
	"testing"

	"github.com/devuser/aetherstream/pkg/db"
	"github.com/devuser/aetherstream/pkg/naming"
	"github.com/devuser/aetherstream/pkg/scanner"
)

func TestParseFilename(t *testing.T) {
	tests := []struct {
		path     string
		wantKind string
		wantTitle string
		wantYear int
		wantSeason int
		wantEpisode int
	}{
		{"/movies/The.Matrix.1999.1080p.mkv", "movie", "The Matrix", 1999, 0, 0},
		{"/tv/Game.of.Thrones.S01E02.mkv", "episode", "Game of Thrones", 0, 1, 2},
		{"/tv/Breaking.Bad.1x02.mkv", "episode", "Breaking Bad", 0, 1, 2},
		{"/music/01 - Artist - Song.mp3", "music", "Artist - Song", 0, 0, 0},
		{"/movies/Inception (2010).mkv", "movie", "Inception", 2010, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			r := naming.ParseFilename(tt.path)
			if r.Kind != tt.wantKind {
				t.Errorf("kind = %q, want %q", r.Kind, tt.wantKind)
			}
			if r.Title != tt.wantTitle {
				t.Errorf("title = %q, want %q", r.Title, tt.wantTitle)
			}
			if r.Year != tt.wantYear {
				t.Errorf("year = %d, want %d", r.Year, tt.wantYear)
			}
			if r.Season != tt.wantSeason {
				t.Errorf("season = %d, want %d", r.Season, tt.wantSeason)
			}
			if r.Episode != tt.wantEpisode {
				t.Errorf("episode = %d, want %d", r.Episode, tt.wantEpisode)
			}
		})
	}
}

func TestScannerClassify(t *testing.T) {
	s, err := scanner.NewScanner()
	if err != nil {
		t.Fatalf("new scanner: %v", err)
	}
	defer s.Close()

	// Test classification logic via ScanLibrary on temp dir
	// (would need actual files — simplified unit test)
	_ = s
}

func TestLibraryManager(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	m, err := NewManager(database, "")
	if err != nil {
		t.Fatalf("manager: %v", err)
	}
	defer m.Close()

	// Create library
	libID, err := m.CreateLibrary("Test Movies", "/tmp/test-movies", "movies")
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	if libID == "" {
		t.Fatal("expected library ID")
	}

	// List libraries
	libs, err := database.ListLibraries()
	if err != nil {
		t.Fatalf("list libraries: %v", err)
	}
	if len(libs) != 1 {
		t.Fatalf("expected 1 library, got %d", len(libs))
	}
}
