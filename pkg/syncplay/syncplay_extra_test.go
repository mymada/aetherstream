package syncplay

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRoom_Found(t *testing.T) {
	m := NewManager()
	m.CreateRoom("r1", "item1", "user1")
	r, ok := m.GetRoom("r1")
	assert.True(t, ok)
	assert.Equal(t, "r1", r.ID)
	assert.Equal(t, "item1", r.ItemID)
	assert.Equal(t, "user1", r.HostUserID)
}

func TestGetRoom_NotFound(t *testing.T) {
	m := NewManager()
	_, ok := m.GetRoom("missing")
	assert.False(t, ok)
}

func TestJoinRoom_NotFound(t *testing.T) {
	m := NewManager()
	ok := m.JoinRoom("missing", "user1")
	assert.False(t, ok)
}

func TestLeaveRoom_NotFound(t *testing.T) {
	m := NewManager()
	ok := m.LeaveRoom("missing", "user1")
	assert.False(t, ok)
}

func TestLeaveRoom_LastMemberDeletesRoom(t *testing.T) {
	m := NewManager()
	m.CreateRoom("r1", "item1", "user1")
	// user1 is the only member — leaving should delete room
	ok := m.LeaveRoom("r1", "user1")
	assert.True(t, ok)
	_, exists := m.GetRoom("r1")
	assert.False(t, exists)
}

func TestLeaveRoom_KeepsRoomWithOtherMembers(t *testing.T) {
	m := NewManager()
	m.CreateRoom("r1", "item1", "user1")
	m.JoinRoom("r1", "user2")
	ok := m.LeaveRoom("r1", "user2")
	assert.True(t, ok)
	_, exists := m.GetRoom("r1")
	assert.True(t, exists, "room should still exist when other members remain")
}

func TestUpdateState_NotFound(t *testing.T) {
	m := NewManager()
	ok := m.UpdateState("missing", "user1", "playing", 10.0)
	assert.False(t, ok)
}

func TestUpdateState_UpdatesFields(t *testing.T) {
	m := NewManager()
	m.CreateRoom("r1", "item1", "host")
	ok := m.UpdateState("r1", "host", "playing", 42.5)
	assert.True(t, ok)
	r, _ := m.GetRoom("r1")
	assert.Equal(t, "playing", r.State)
	assert.Equal(t, 42.5, r.Position)
}

func TestListRooms_Empty(t *testing.T) {
	m := NewManager()
	rooms := m.ListRooms()
	assert.Nil(t, rooms)
}

func TestListRooms_WithRooms(t *testing.T) {
	m := NewManager()
	m.CreateRoom("r1", "item1", "user1")
	m.CreateRoom("r2", "item2", "user2")
	rooms := m.ListRooms()
	assert.Len(t, rooms, 2)
}

func TestListRooms_ContainsFields(t *testing.T) {
	m := NewManager()
	m.CreateRoom("r1", "item1", "user1")
	rooms := m.ListRooms()
	require.Len(t, rooms, 1)
	assert.Equal(t, "r1", rooms[0]["id"])
	assert.Equal(t, "item1", rooms[0]["item_id"])
	assert.Equal(t, "user1", rooms[0]["host"])
	assert.Equal(t, "paused", rooms[0]["state"])
}

func TestBroadcastToRoom_NotFound(t *testing.T) {
	m := NewManager()
	assert.NotPanics(t, func() {
		m.BroadcastToRoom("missing", []byte("payload"))
	})
}

func TestBroadcastToRoom_ExistingRoom(t *testing.T) {
	m := NewManager()
	m.CreateRoom("r1", "item1", "user1")
	assert.NotPanics(t, func() {
		m.BroadcastToRoom("r1", []byte(`{"type":"ping"}`))
	})
}

// --- HandleWSMessage ---

func TestHandleWSMessage_Create(t *testing.T) {
	m := NewManager()
	msg, _ := json.Marshal(map[string]interface{}{
		"type": "syncplay_create", "room_id": "r1", "item_id": "item1",
	})
	m.HandleWSMessage("user1", msg)
	_, ok := m.GetRoom("r1")
	assert.True(t, ok)
}

func TestHandleWSMessage_Join(t *testing.T) {
	m := NewManager()
	m.CreateRoom("r1", "item1", "user1")
	msg, _ := json.Marshal(map[string]interface{}{
		"type": "syncplay_join", "room_id": "r1",
	})
	m.HandleWSMessage("user2", msg)
	r, _ := m.GetRoom("r1")
	r.mu.RLock()
	_, in := r.members["user2"]
	r.mu.RUnlock()
	assert.True(t, in)
}

func TestHandleWSMessage_Leave(t *testing.T) {
	m := NewManager()
	m.CreateRoom("r1", "item1", "user1")
	m.JoinRoom("r1", "user2")
	msg, _ := json.Marshal(map[string]interface{}{
		"type": "syncplay_leave", "room_id": "r1",
	})
	m.HandleWSMessage("user2", msg)
	r, ok := m.GetRoom("r1")
	require.True(t, ok)
	r.mu.RLock()
	_, in := r.members["user2"]
	r.mu.RUnlock()
	assert.False(t, in)
}

func TestHandleWSMessage_State(t *testing.T) {
	m := NewManager()
	m.CreateRoom("r1", "item1", "user1")
	msg, _ := json.Marshal(map[string]interface{}{
		"type": "syncplay_state", "room_id": "r1", "state": "playing", "position": 99.9,
	})
	m.HandleWSMessage("user1", msg)
	r, _ := m.GetRoom("r1")
	assert.Equal(t, "playing", r.State)
	assert.Equal(t, 99.9, r.Position)
}

func TestHandleWSMessage_InvalidJSON(t *testing.T) {
	m := NewManager()
	assert.NotPanics(t, func() {
		m.HandleWSMessage("user1", []byte("{invalid"))
	})
}

func TestHandleWSMessage_UnknownType(t *testing.T) {
	m := NewManager()
	msg, _ := json.Marshal(map[string]interface{}{
		"type": "syncplay_unknown", "room_id": "r1",
	})
	assert.NotPanics(t, func() {
		m.HandleWSMessage("user1", msg)
	})
}

func TestCreateRoom_InitialState(t *testing.T) {
	m := NewManager()
	r := m.CreateRoom("r1", "item1", "host")
	assert.Equal(t, "paused", r.State)
	assert.Equal(t, 0.0, r.Position)
	assert.False(t, r.LastUpdate.IsZero())
}
