package models

import "time"

// PlaybackProgress represents the current playback position (resume point) for a user/item pair.
type PlaybackProgress struct {
	UserID           string    `json:"userId"`
	ItemID           string    `json:"itemId"`
	PositionSeconds  float64   `json:"positionSeconds"`
	DurationSeconds  float64   `json:"durationSeconds"`
	PercentComplete  float64   `json:"percentComplete"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// WatchHistory represents a completed or partial watch event for an item by a user.
type WatchHistory struct {
	ID              int       `json:"id"`
	UserID          string    `json:"userId"`
	ItemID          string    `json:"itemId"`
	PositionSeconds float64   `json:"positionSeconds"`
	DurationSeconds float64   `json:"durationSeconds"`
	Watched         bool      `json:"watched"`
	WatchedAt       time.Time `json:"watchedAt"`
}
