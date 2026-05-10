package db

import "time"

// User represents a user in the system.
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

// Library represents a media library.
type Library struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	MediaType string    `json:"mediaType"`
	CreatedAt time.Time `json:"createdAt"`
}

// Item represents a media item.
type Item struct {
	ID               string    `json:"id"`
	LibraryID        string    `json:"libraryId"`
	Path             string    `json:"path"`
	Name             string    `json:"name"`
	MediaType        string    `json:"mediaType"`
	Container        string    `json:"container"`
	SizeBytes        int64     `json:"sizeBytes"`
	DurationSeconds  float64   `json:"durationSeconds"`
	Width            int       `json:"width"`
	Height           int       `json:"height"`
	VideoCodec       string    `json:"videoCodec"`
	AudioCodec       string    `json:"audioCodec"`
	CreatedAt        time.Time `json:"createdAt"`
}

// Collection represents a user collection/playlist.
type Collection struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"createdAt"`
}

// Activity represents an activity log entry.
type Activity struct {
	ID        int       `json:"id"`
	UserID    string    `json:"userId"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	CreatedAt time.Time `json:"createdAt"`
}
