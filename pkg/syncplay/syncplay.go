package syncplay

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/devuser/aetherstream/pkg/ws"
	"github.com/rs/zerolog/log"
)

// Room represents a synchronized playback session for multiple users.
type Room struct {
	ID           string
	ItemID       string
	HostUserID   string
	State        string // "playing" | "paused" | "buffering"
	Position     float64 // seconds
	LastUpdate   time.Time
	mu           sync.RWMutex
	members      map[string]bool // userIDs
}

// Manager holds all active SyncPlay rooms.
type Manager struct {
	rooms map[string]*Room
	mu    sync.RWMutex
}

// NewManager creates a SyncPlay manager.
func NewManager() *Manager {
	return &Manager{
		rooms: make(map[string]*Room),
	}
}

// CreateRoom starts a new SyncPlay room.
func (m *Manager) CreateRoom(roomID, itemID, hostUserID string) *Room {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := &Room{
		ID:         roomID,
		ItemID:     itemID,
		HostUserID: hostUserID,
		State:      "paused",
		Position:   0,
		LastUpdate: time.Now(),
		members:    map[string]bool{hostUserID: true},
	}
	m.rooms[roomID] = r
	return r
}

// GetRoom returns a room by ID.
func (m *Manager) GetRoom(roomID string) (*Room, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.rooms[roomID]
	return r, ok
}

// JoinRoom adds a user to a room.
func (m *Manager) JoinRoom(roomID, userID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rooms[roomID]
	if !ok {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.members[userID] = true
	return true
}

// LeaveRoom removes a user from a room.
func (m *Manager) LeaveRoom(roomID, userID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rooms[roomID]
	if !ok {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.members, userID)
	if len(r.members) == 0 {
		delete(m.rooms, roomID)
	}
	return true
}

// UpdateState sets playback state and broadcasts to members.
func (m *Manager) UpdateState(roomID, userID, state string, position float64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rooms[roomID]
	if !ok {
		return false
	}
	r.mu.Lock()
	r.State = state
	r.Position = position
	r.LastUpdate = time.Now()
	members := make([]string, 0, len(r.members))
	for uid := range r.members {
		members = append(members, uid)
	}
	r.mu.Unlock()

	msg := map[string]interface{}{
		"type":     "syncplay_state",
		"room_id":  roomID,
		"user_id":  userID,
		"state":    state,
		"position": position,
		"ts":       time.Now().UnixMilli(),
	}
	payload, _ := json.Marshal(msg)
	for _, uid := range members {
		if uid != userID {
			ws.BroadcastToUser(uid, payload)
		}
	}
	return true
}

// BroadcastToRoom sends a custom message to all room members via WebSocket.
func (m *Manager) BroadcastToRoom(roomID string, payload []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.rooms[roomID]
	if !ok {
		return
	}
	r.mu.RLock()
	members := make([]string, 0, len(r.members))
	for uid := range r.members {
		members = append(members, uid)
	}
	r.mu.RUnlock()
	for _, uid := range members {
		ws.BroadcastToUser(uid, payload)
	}
}

// ListRooms returns a snapshot of all active rooms.
func (m *Manager) ListRooms() []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []map[string]interface{}
	for _, r := range m.rooms {
		r.mu.RLock()
		out = append(out, map[string]interface{}{
			"id":         r.ID,
			"item_id":    r.ItemID,
			"host":       r.HostUserID,
			"state":      r.State,
			"position":   r.Position,
			"members":    len(r.members),
			"last_update": r.LastUpdate,
		})
		r.mu.RUnlock()
	}
	return out
}

// HandleWSMessage processes an incoming WebSocket message for SyncPlay.
func (m *Manager) HandleWSMessage(userID string, raw []byte) {
	var msg struct {
		Type     string  `json:"type"`
		RoomID   string  `json:"room_id"`
		ItemID   string  `json:"item_id"`
		State    string  `json:"state"`
		Position float64 `json:"position"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		log.Warn().Err(err).Str("user", userID).Msg("syncplay invalid msg")
		return
	}
	switch msg.Type {
	case "syncplay_create":
		m.CreateRoom(msg.RoomID, msg.ItemID, userID)
	case "syncplay_join":
		m.JoinRoom(msg.RoomID, userID)
	case "syncplay_leave":
		m.LeaveRoom(msg.RoomID, userID)
	case "syncplay_state":
		m.UpdateState(msg.RoomID, userID, msg.State, msg.Position)
	}
}
