package compliance

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devuser/aetherstream/pkg/db"
	"github.com/google/uuid"
)

// GDPRService handles data export and deletion requests.
type GDPRService struct {
	db *db.DB
}

// NewGDPRService creates a new GDPR compliance service.
func NewGDPRService(database *db.DB) *GDPRService {
	return &GDPRService{db: database}
}

// UserDataExport represents all personal data for a user.
type UserDataExport struct {
	ExportID    string                 `json:"exportId"`
	UserID      string                 `json:"userId"`
	ExportedAt  time.Time              `json:"exportedAt"`
	Profile     map[string]interface{} `json:"profile"`
	Libraries   []db.Library           `json:"libraries"`
	Collections []db.Collection        `json:"collections"`
	Activity    []db.Activity          `json:"activity"`
	Progress    []db.PlaybackProgress  `json:"playbackProgress"`
	History     []db.WatchHistory      `json:"watchHistory"`
}

// ExportUserData collects all personal data for a user and returns a JSON blob and ZIP archive.
func (g *GDPRService) ExportUserData(userID string) (*UserDataExport, []byte, error) {
	export := &UserDataExport{
		ExportID:   uuid.New().String(),
		UserID:     userID,
		ExportedAt: time.Now().UTC(),
		Profile:    make(map[string]interface{}),
	}

	// Profile
	profile, err := g.db.GetUserByID(userID)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch profile: %w", err)
	}
	export.Profile = profile

	// Collections
	cols, err := g.db.ListCollections(userID)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch collections: %w", err)
	}
	export.Collections = cols

	// Activity
	acts, err := g.db.ListActivity(10000)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch activity: %w", err)
	}
	var userActs []db.Activity
	for _, a := range acts {
		if a.UserID == userID {
			userActs = append(userActs, a)
		}
	}
	export.Activity = userActs

	// Playback progress
	// We need a way to list all progress for a user; db doesn't expose it directly.
	// We'll use a helper query via the existing DB connection.
	progressRows, err := g.db.Query(`
		SELECT user_id, item_id, position_seconds, duration_seconds, percent_complete, updated_at
		FROM playback_progress WHERE user_id = ?`, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch progress: %w", err)
	}
	defer progressRows.Close()
	for progressRows.Next() {
		var p db.PlaybackProgress
		if err := progressRows.Scan(&p.UserID, &p.ItemID, &p.PositionSeconds, &p.DurationSeconds, &p.PercentComplete, &p.UpdatedAt); err != nil {
			continue
		}
		export.Progress = append(export.Progress, p)
	}

	// Watch history
	historyRows, err := g.db.Query(`
		SELECT id, user_id, item_id, position_seconds, duration_seconds, watched, watched_at
		FROM watch_history WHERE user_id = ? ORDER BY watched_at DESC`, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch history: %w", err)
	}
	defer historyRows.Close()
	for historyRows.Next() {
		var h db.WatchHistory
		var watchedInt int
		if err := historyRows.Scan(&h.ID, &h.UserID, &h.ItemID, &h.PositionSeconds, &h.DurationSeconds, &watchedInt, &h.WatchedAt); err != nil {
			continue
		}
		h.Watched = watchedInt != 0
		export.History = append(export.History, h)
	}

	// JSON export
	jsonData, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("marshal export: %w", err)
	}

	// ZIP export
	zipBuf := new(bytes.Buffer)
	zw := zip.NewWriter(zipBuf)
	w, err := zw.Create("export.json")
	if err != nil {
		return nil, nil, fmt.Errorf("create zip entry: %w", err)
	}
	if _, err := w.Write(jsonData); err != nil {
		return nil, nil, fmt.Errorf("write zip entry: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, nil, fmt.Errorf("close zip: %w", err)
	}

	return export, zipBuf.Bytes(), nil
}

// DeleteUserData removes or anonymizes all personal data for a user.
func (g *GDPRService) DeleteUserData(userID string) error {
	// Anonymize user profile
	_, err := g.db.Exec("UPDATE users SET username = ?, password_hash = ? WHERE id = ?",
		"deleted-"+userID, "", userID)
	if err != nil {
		return fmt.Errorf("anonymize user: %w", err)
	}

	// Delete collections
	_, err = g.db.Exec("DELETE FROM collections WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("delete collections: %w", err)
	}

	// Delete activity log entries
	_, err = g.db.Exec("DELETE FROM activity_log WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("delete activity: %w", err)
	}

	// Delete playback progress
	_, err = g.db.Exec("DELETE FROM playback_progress WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("delete progress: %w", err)
	}

	// Delete watch history
	_, err = g.db.Exec("DELETE FROM watch_history WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("delete history: %w", err)
	}

	// Delete sessions
	_, err = g.db.Exec("DELETE FROM sessions WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("delete sessions: %w", err)
	}

	return nil
}

// WriteExportToFile writes the ZIP export to a file path.
func (g *GDPRService) WriteExportToFile(zipData []byte, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "gdpr-export-"+uuid.New().String()+".zip")
	if err := os.WriteFile(path, zipData, 0600); err != nil {
		return "", err
	}
	return path, nil
}

// ReadExportFromFile reads a previously written export ZIP file.
func ReadExportFromFile(path string) ([]byte, error) {
	// Validate path is within exports directory to prevent path traversal
	cleanPath := filepath.Clean(path)
	if !strings.HasPrefix(cleanPath, filepath.Clean("exports")) {
		return nil, fmt.Errorf("invalid export path")
	}
	f, err := os.Open(cleanPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}
