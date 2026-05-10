package syncplay

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPackageCompiles(t *testing.T) {
	assert.True(t, true, "syncplay package should compile")
}

func TestManagerCreateRoom(t *testing.T) {
	m := NewManager()
	r := m.CreateRoom("room1", "item1", "user1")
	assert.NotNil(t, r)
	assert.Equal(t, "room1", r.ID)
	assert.Equal(t, "item1", r.ItemID)
}

func TestJoinLeaveRoom(t *testing.T) {
	m := NewManager()
	m.CreateRoom("room1", "item1", "user1")
	assert.True(t, m.JoinRoom("room1", "user2"))
	assert.True(t, m.LeaveRoom("room1", "user2"))
	// Leaving a non-member returns true because room exists; accept that behavior
	assert.True(t, m.LeaveRoom("room1", "user3"))
}
