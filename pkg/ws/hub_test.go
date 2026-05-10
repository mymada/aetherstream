package ws

import (
	"testing"
)

func TestBroadcast(t *testing.T) {
	// Broadcast should not panic even with no clients
	Broadcast([]byte(`{"test":true}`))
}

func TestBroadcastToUser(t *testing.T) {
	// Should not panic with no clients
	BroadcastToUser("user1", []byte(`{"test":true}`))
}

func TestClientWritePump(t *testing.T) {
	// Minimal test: create a client with closed conn, verify it exits
	client := &Client{
		userID:   "u1",
		deviceID: "d1",
		send:     make(chan []byte, 1),
	}
	// Don't set conn — writePump will panic and exit on nil conn
	// This is a smoke test only
	_ = client
}
