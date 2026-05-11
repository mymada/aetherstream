package playback

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/devuser/aetherstream/pkg/db"
)

// Store defines persistence operations for playback sessions.
type Store interface {
	CreateSession(userID, itemID, deviceID string) (*PlaybackSession, error)
	GetSession(id string) (*PlaybackSession, error)
	UpdateSession(session *PlaybackSession) error
	DeleteSession(id string) error
	ListActiveSessions(userID string) ([]PlaybackSession, error)
}

// Ensure DB implements Store at compile time.
var _ Store = (*DB)(nil)

// DB wraps the application's database with playback-specific operations.
type DB struct {
	*db.DB
}

// NewStore creates a playback store backed by the shared database.
func NewStore(database *db.DB) *DB {
	return &DB{database}
}

// Migrate creates the playback_sessions table if it does not exist.
func (d *DB) Migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS playback_sessions (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
	device_id TEXT NOT NULL,
	state TEXT NOT NULL DEFAULT 'stopped',
	position INTEGER NOT NULL DEFAULT 0,
	volume INTEGER NOT NULL DEFAULT 100,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_playback_sessions_user ON playback_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_playback_sessions_item ON playback_sessions(item_id);
CREATE INDEX IF NOT EXISTS idx_playback_sessions_state ON playback_sessions(state);
`
	if _, err := d.Exec(schema); err != nil {
		return fmt.Errorf("migrate playback_sessions: %w", err)
	}
	return nil
}

// CreateSession inserts a new playback session with state "playing".
func (d *DB) CreateSession(userID, itemID, deviceID string) (*PlaybackSession, error) {
	session := &PlaybackSession{
		ID:        uuid.New().String(),
		UserID:    userID,
		ItemID:    itemID,
		DeviceID:  deviceID,
		State:     StatePlaying,
		Position:  0,
		Volume:    100,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	_, err := d.Exec(
		`INSERT INTO playback_sessions(id, user_id, item_id, device_id, state, position, volume, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.UserID, session.ItemID, session.DeviceID,
		string(session.State), session.Position, session.Volume, session.CreatedAt, session.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return session, nil
}

// GetSession fetches a session by ID.
func (d *DB) GetSession(id string) (*PlaybackSession, error) {
	row := d.QueryRow(
		`SELECT id, user_id, item_id, device_id, state, position, volume, created_at, updated_at
		 FROM playback_sessions WHERE id = ?`, id)
	var s PlaybackSession
	var stateStr string
	err := row.Scan(&s.ID, &s.UserID, &s.ItemID, &s.DeviceID, &stateStr, &s.Position, &s.Volume, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found")
		}
		return nil, err
	}
	s.State = PlaybackState(stateStr)
	return &s, nil
}

// UpdateSession persists changes to an existing session.
func (d *DB) UpdateSession(session *PlaybackSession) error {
	session.UpdatedAt = time.Now().UTC()
	_, err := d.Exec(
		`UPDATE playback_sessions
		 SET state = ?, position = ?, volume = ?, updated_at = ?
		 WHERE id = ?`,
		string(session.State), session.Position, session.Volume, session.UpdatedAt, session.ID,
	)
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}
	return nil
}

// DeleteSession removes a session by ID.
func (d *DB) DeleteSession(id string) error {
	_, err := d.Exec("DELETE FROM playback_sessions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// ListActiveSessions returns sessions for a user that are not stopped.
func (d *DB) ListActiveSessions(userID string) ([]PlaybackSession, error) {
	rows, err := d.Query(
		`SELECT id, user_id, item_id, device_id, state, position, volume, created_at, updated_at
		 FROM playback_sessions
		 WHERE user_id = ? AND state != ?
		 ORDER BY updated_at DESC`,
		userID, string(StateStopped),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []PlaybackSession
	for rows.Next() {
		var s PlaybackSession
		var stateStr string
		if err := rows.Scan(&s.ID, &s.UserID, &s.ItemID, &s.DeviceID, &stateStr, &s.Position, &s.Volume, &s.CreatedAt, &s.UpdatedAt); err != nil {
			continue
		}
		s.State = PlaybackState(stateStr)
		sessions = append(sessions, s)
	}
	return sessions, nil
}
