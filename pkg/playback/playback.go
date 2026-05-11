package playback

import "time"

// PlaybackState represents the current playback state.
type PlaybackState string

const (
	StatePlaying PlaybackState = "playing"
	StatePaused  PlaybackState = "paused"
	StateStopped PlaybackState = "stopped"
)

// PlaybackCommand represents a control command sent to a session.
type PlaybackCommand string

const (
	CommandPlay    PlaybackCommand = "play"
	CommandPause   PlaybackCommand = "pause"
	CommandStop    PlaybackCommand = "stop"
	CommandSeek    PlaybackCommand = "seek"
	CommandVolume  PlaybackCommand = "volume"
)

// PlaybackSession represents an active playback session.
type PlaybackSession struct {
	ID        string        `json:"id"`
	UserID    string        `json:"userId"`
	ItemID    string        `json:"itemId"`
	DeviceID  string        `json:"deviceId"`
	State     PlaybackState `json:"state"`
	Position  int64         `json:"position"` // milliseconds
	Volume    int           `json:"volume"`   // 0-100
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
}

// PlaybackEvent is emitted after each command for real-time forwarding (WebSocket).
type PlaybackEvent struct {
	SessionID string          `json:"sessionId"`
	Command   PlaybackCommand `json:"command"`
	Position  int64           `json:"position"` // milliseconds
	Volume    int             `json:"volume"`   // 0-100
	Timestamp time.Time       `json:"timestamp"`
}
