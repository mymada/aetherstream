package ws

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func makeClient(userID string) *Client {
	return &Client{
		userID:   userID,
		deviceID: "",
		send:     make(chan []byte, 16),
	}
}

func TestForwardCommand_NoSession(t *testing.T) {
	ok := ForwardCommand("nonexistent-device", PlaybackMessage{Type: "command", Command: "play"})
	assert.False(t, ok)
}

func TestForwardEvent_NoSession(t *testing.T) {
	ok := ForwardEvent("nonexistent-device", PlaybackMessage{Type: "event", State: "paused"})
	assert.False(t, ok)
}

func TestPairController_SessionNotFound(t *testing.T) {
	ctrl := makeClient("user-pc")
	ok := PairController(ctrl, "nonexistent-device-id")
	assert.False(t, ok)
	// Should send an error message to the controller
	select {
	case raw := <-ctrl.send:
		var m PlaybackMessage
		assert.NoError(t, json.Unmarshal(raw, &m))
		assert.Equal(t, "error", m.Type)
	default:
		t.Fatal("expected error message sent to controller")
	}
}

func TestUnregisterPlayback_NilClient(t *testing.T) {
	assert.NotPanics(t, func() {
		UnregisterPlayback(nil)
	})
}

func TestUnregisterPlayback_NoDeviceID(t *testing.T) {
	client := makeClient("user-x")
	client.deviceID = "" // not registered
	assert.NotPanics(t, func() {
		UnregisterPlayback(client)
	})
}

func TestHandlePlaybackMessage_EventWithDeviceID(t *testing.T) {
	recv := makeClient("recv-user")
	devID := RegisterReceiver(recv)
	<-recv.send // drain pair message

	// Register a controller to receive the forwarded event
	ctrl := makeClient("ctrl-user")
	PairController(ctrl, devID)
	<-ctrl.send // drain paired message

	// Send an event from the receiver
	msg := []byte(`{"type":"event","state":"playing","position":5000}`)
	HandlePlaybackMessage(recv, msg)

	select {
	case raw := <-ctrl.send:
		var m PlaybackMessage
		assert.NoError(t, json.Unmarshal(raw, &m))
		assert.Equal(t, "event", m.Type)
		assert.Equal(t, "playing", m.State)
	default:
		t.Fatal("expected event forwarded to controller")
	}

	UnregisterPlayback(recv)
}

func TestHandlePlaybackMessage_InvalidJSON(t *testing.T) {
	client := makeClient("user-ij")
	HandlePlaybackMessage(client, []byte("{invalid"))
	select {
	case raw := <-client.send:
		var m PlaybackMessage
		assert.NoError(t, json.Unmarshal(raw, &m))
		assert.Equal(t, "error", m.Type)
	default:
		t.Fatal("expected error message for invalid JSON")
	}
}

func TestHandlePlaybackMessage_UnknownType(t *testing.T) {
	client := makeClient("user-ut")
	msg := []byte(`{"type":"unknown_type"}`)
	// Should not panic or send any message
	assert.NotPanics(t, func() {
		HandlePlaybackMessage(client, msg)
	})
}

func TestHandlePlaybackMessage_EventNoDeviceID(t *testing.T) {
	client := makeClient("user-end")
	client.deviceID = "" // not registered as receiver
	msg := []byte(`{"type":"event","state":"playing"}`)
	// Should silently return (no device_id)
	assert.NotPanics(t, func() {
		HandlePlaybackMessage(client, msg)
	})
	assert.Empty(t, client.send)
}

func TestRegisterReceiver_UniqueDeviceIDs(t *testing.T) {
	c1 := makeClient("user-r1")
	c2 := makeClient("user-r2")
	id1 := RegisterReceiver(c1)
	id2 := RegisterReceiver(c2)
	assert.NotEqual(t, id1, id2)
	<-c1.send
	<-c2.send
	UnregisterPlayback(c1)
	UnregisterPlayback(c2)
}

func TestForwardCommand_WithPairedReceiver(t *testing.T) {
	recv := makeClient("recv-fc")
	devID := RegisterReceiver(recv)
	<-recv.send // drain pair

	ctrl := makeClient("ctrl-fc")
	PairController(ctrl, devID)
	<-ctrl.send // drain paired

	cmd := PlaybackMessage{Type: "command", Command: "seek", Position: 30000}
	ok := ForwardCommand(devID, cmd)
	assert.True(t, ok)

	select {
	case raw := <-recv.send:
		var m PlaybackMessage
		assert.NoError(t, json.Unmarshal(raw, &m))
		assert.Equal(t, "seek", m.Command)
	default:
		t.Fatal("expected command forwarded to receiver")
	}

	UnregisterPlayback(recv)
}
