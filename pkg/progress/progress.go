package progress

import (
	"database/sql"
	"fmt"
	"time"
)

// Service provides playback progress and watch history operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new progress service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Record represents a user's playback position for a single item.
type Record struct {
	UserID          string    `json:"userId"`
	ItemID          string    `json:"itemId"`
	PositionSeconds float64   `json:"positionSeconds"`
	DurationSeconds float64   `json:"durationSeconds"`
	PercentComplete float64   `json:"percentComplete"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// WatchEntry represents a watch history entry.
type WatchEntry struct {
	ID              int       `json:"id"`
	UserID          string    `json:"userId"`
	ItemID          string    `json:"itemId"`
	PositionSeconds float64   `json:"positionSeconds"`
	DurationSeconds float64   `json:"durationSeconds"`
	Watched         bool      `json:"watched"`
	WatchedAt       time.Time `json:"watchedAt"`
}

// SaveProgress upserts the resume point for a user/item pair.
func (s *Service) SaveProgress(userID, itemID string, positionSeconds, durationSeconds, percentComplete float64) error {
	if userID == "" || itemID == "" {
		return fmt.Errorf("userID and itemID are required")
	}
	if durationSeconds <= 0 {
		return fmt.Errorf("duration_seconds must be positive")
	}
	if positionSeconds < 0 {
		positionSeconds = 0
	}
	if percentComplete < 0 {
		percentComplete = 0
	}
	if percentComplete > 100 {
		percentComplete = 100
	}

	_, err := s.db.Exec(`
		INSERT INTO playback_progress(user_id, item_id, position_seconds, duration_seconds, percent_complete, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id, item_id) DO UPDATE SET
			position_seconds = excluded.position_seconds,
			duration_seconds = excluded.duration_seconds,
			percent_complete = excluded.percent_complete,
			updated_at = CURRENT_TIMESTAMP`,
		userID, itemID, positionSeconds, durationSeconds, percentComplete,
	)
	return err
}

// GetProgress returns the resume point for a user/item pair.
func (s *Service) GetProgress(userID, itemID string) (*Record, error) {
	if userID == "" || itemID == "" {
		return nil, fmt.Errorf("userID and itemID are required")
	}
	row := s.db.QueryRow(`
		SELECT user_id, item_id, position_seconds, duration_seconds, percent_complete, updated_at
		FROM playback_progress
		WHERE user_id = ? AND item_id = ?`, userID, itemID)
	var r Record
	err := row.Scan(&r.UserID, &r.ItemID, &r.PositionSeconds, &r.DurationSeconds, &r.PercentComplete, &r.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("playback progress not found")
		}
		return nil, err
	}
	return &r, nil
}

// DeleteProgress removes the resume point for a user/item pair.
func (s *Service) DeleteProgress(userID, itemID string) error {
	if userID == "" || itemID == "" {
		return fmt.Errorf("userID and itemID are required")
	}
	_, err := s.db.Exec("DELETE FROM playback_progress WHERE user_id = ? AND item_id = ?", userID, itemID)
	return err
}

// ListProgressByUser returns all resume points for a user, newest first.
func (s *Service) ListProgressByUser(userID string, limit int) ([]Record, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID is required")
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT user_id, item_id, position_seconds, duration_seconds, percent_complete, updated_at
		FROM playback_progress
		WHERE user_id = ?
		ORDER BY updated_at DESC
		LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var r Record
		if err := rows.Scan(&r.UserID, &r.ItemID, &r.PositionSeconds, &r.DurationSeconds, &r.PercentComplete, &r.UpdatedAt); err != nil {
			continue
		}
		records = append(records, r)
	}
	return records, nil
}

// MarkWatched records a watch history entry and clears the resume point.
func (s *Service) MarkWatched(userID, itemID string, positionSeconds, durationSeconds float64) error {
	if userID == "" || itemID == "" {
		return fmt.Errorf("userID and itemID are required")
	}
	if durationSeconds <= 0 {
		return fmt.Errorf("duration_seconds must be positive")
	}
	if positionSeconds < 0 {
		positionSeconds = 0
	}
	watchedInt := 1
	_, err := s.db.Exec(`
		INSERT INTO watch_history(user_id, item_id, position_seconds, duration_seconds, watched, watched_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id, item_id) DO UPDATE SET
			position_seconds = excluded.position_seconds,
			duration_seconds = excluded.duration_seconds,
			watched = excluded.watched,
			watched_at = CURRENT_TIMESTAMP`,
		userID, itemID, positionSeconds, durationSeconds, watchedInt,
	)
	if err != nil {
		return err
	}
	// Clear resume point when fully watched
	if positionSeconds >= durationSeconds*0.9 {
		_ = s.DeleteProgress(userID, itemID)
	}
	return nil
}

// MarkUnwatched records an unwatched state for a user/item pair.
func (s *Service) MarkUnwatched(userID, itemID string) error {
	if userID == "" || itemID == "" {
		return fmt.Errorf("userID and itemID are required")
	}
	_, err := s.db.Exec(`
		INSERT INTO watch_history(user_id, item_id, position_seconds, duration_seconds, watched, watched_at)
		VALUES (?, ?, 0, 0, 0, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id, item_id) DO UPDATE SET
			position_seconds = 0,
			duration_seconds = 0,
			watched = 0,
			watched_at = CURRENT_TIMESTAMP`,
		userID, itemID,
	)
	if err != nil {
		return err
	}
	_ = s.DeleteProgress(userID, itemID)
	return nil
}

// GetWatchEntry returns the watch history entry for a user/item pair.
func (s *Service) GetWatchEntry(userID, itemID string) (*WatchEntry, error) {
	if userID == "" || itemID == "" {
		return nil, fmt.Errorf("userID and itemID are required")
	}
	row := s.db.QueryRow(`
		SELECT id, user_id, item_id, position_seconds, duration_seconds, watched, watched_at
		FROM watch_history
		WHERE user_id = ? AND item_id = ?`, userID, itemID)
	var w WatchEntry
	var watchedInt int
	err := row.Scan(&w.ID, &w.UserID, &w.ItemID, &w.PositionSeconds, &w.DurationSeconds, &watchedInt, &w.WatchedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("watch history not found")
		}
		return nil, err
	}
	w.Watched = watchedInt != 0
	return &w, nil
}

// ListWatchHistoryByUser returns recent watch history for a user.
func (s *Service) ListWatchHistoryByUser(userID string, limit int) ([]WatchEntry, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID is required")
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT id, user_id, item_id, position_seconds, duration_seconds, watched, watched_at
		FROM watch_history
		WHERE user_id = ?
		ORDER BY watched_at DESC
		LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []WatchEntry
	for rows.Next() {
		var w WatchEntry
		var watchedInt int
		if err := rows.Scan(&w.ID, &w.UserID, &w.ItemID, &w.PositionSeconds, &w.DurationSeconds, &watchedInt, &w.WatchedAt); err != nil {
			continue
		}
		w.Watched = watchedInt != 0
		history = append(history, w)
	}
	return history, nil
}

// SyncReport returns combined playback progress + watch history for a user.
func (s *Service) SyncReport(userID string) (map[string]interface{}, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID is required")
	}
	progress, err := s.ListProgressByUser(userID, 1000)
	if err != nil {
		return nil, err
	}
	history, err := s.ListWatchHistoryByUser(userID, 1000)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"playbackProgress": progress,
		"watchHistory":     history,
	}, nil
}
