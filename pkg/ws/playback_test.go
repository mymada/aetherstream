package ws

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateDeviceID(t *testing.T) {
	id1 := GenerateDeviceID()
	id2 := GenerateDeviceID()
	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "tv-")
}

func TestPlaybackMessageMarshal(t *testing.T) {
	msg := PlaybackMessage{
		Type:     "command",
		Command:  "play",
		Position: 12000,
		Volume:   80,
	}
	b, err := json.Marshal(msg)
	assert.NoError(t, err)
	assert.Contains(t, string(b), `"type":"command"`)
	assert.Contains(t, string(b), `"command":"play"`)
}

func TestPlaybackSessionPairing(t *testing.T) {
	// Simulate a receiver client
	recv := &Client{
		userID:   "user1",
		deviceID: "",
		send:     make(chan []byte, 10),
	}

	devID := RegisterReceiver(recv)
	assert.NotEmpty(t, devID)
	assert.Equal(t, devID, recv.deviceID)

	// Read the "pair" message sent to receiver
	select {
	case raw := <-recv.send:
		var msg PlaybackMessage
		err := json.Unmarshal(raw, &msg)
		assert.NoError(t, err)
		assert.Equal(t, "pair", msg.Type)
		assert.Equal(t, devID, msg.DeviceID)
	default:
		t.Fatal("expected pair message")
	}

	// Simulate a controller client
	ctrl := &Client{
		userID:   "user1",
		deviceID: "",
		send:     make(chan []byte, 10),
	}

	ok := PairController(ctrl, devID)
	assert.True(t, ok)
	assert.Equal(t, devID, ctrl.deviceID)

	// Read the "paired" message sent to controller
	select {
	case raw := <-ctrl.send:
		var msg PlaybackMessage
		err := json.Unmarshal(raw, &msg)
		assert.NoError(t, err)
		assert.Equal(t, "paired", msg.Type)
		assert.Equal(t, devID, msg.DeviceID)
	default:
		t.Fatal("expected paired message")
	}

	// Forward a command from controller to receiver
	cmd := PlaybackMessage{Type: "command", Command: "pause", Position: 5000}
	sent := ForwardCommand(devID, cmd)
	assert.True(t, sent)

	select {
	case raw := <-recv.send:
		var msg PlaybackMessage
		err := json.Unmarshal(raw, &msg)
		assert.NoError(t, err)
		assert.Equal(t, "command", msg.Type)
		assert.Equal(t, "pause", msg.Command)
		assert.Equal(t, int64(5000), msg.Position)
	default:
		t.Fatal("expected command forwarded to receiver")
	}

	// Forward an event from receiver to controller
	evt := PlaybackMessage{Type: "event", State: "playing", Position: 12000, Volume: 80}
	sent = ForwardEvent(devID, evt)
	assert.True(t, sent)

	select {
	case raw := <-ctrl.send:
		var msg PlaybackMessage
		err := json.Unmarshal(raw, &msg)
		assert.NoError(t, err)
		assert.Equal(t, "event", msg.Type)
		assert.Equal(t, "playing", msg.State)
	default:
		t.Fatal("expected event forwarded to controller")
	}

	// Cleanup: unregister controller
	UnregisterPlayback(ctrl)
	ph := getPlaybackHub()
	ph.mu.RLock()
	session := ph.sessions[devID]
	ph.mu.RUnlock()
	assert.NotNil(t, session)
	session.mu.RLock()
	assert.Nil(t, session.Controller)
	session.mu.RUnlock()

	// Cleanup: unregister receiver
	UnregisterPlayback(recv)
	ph.mu.RLock()
	_, exists := ph.sessions[devID]
	ph.mu.RUnlock()
	assert.False(t, exists)
}

func TestForwardEventNoController(t *testing.T) {
	recv := &Client{
		userID:   "user2",
		deviceID: "",
		send:     make(chan []byte, 10),
	}
	devID := RegisterReceiver(recv)

	// Drain pair message
	<-recv.send

	evt := PlaybackMessage{Type: "event", State: "paused"}
	sent := ForwardEvent(devID, evt)
	assert.False(t, sent)

	UnregisterPlayback(recv)
}

func TestHandlePlaybackMessagePairMissingDeviceID(t *testing.T) {
	ctrl := &Client{
		userID:   "user3",
		deviceID: "",
		send:     make(chan []byte, 10),
	}
	msg := []byte(`{"type":"pair"}`)
	HandlePlaybackMessage(ctrl, msg)

	select {
	case raw := <-ctrl.send:
		var m PlaybackMessage
		err := json.Unmarshal(raw, &m)
		assert.NoError(t, err)
		assert.Equal(t, "error", m.Type)
	default:
		t.Fatal("expected error message")
	}
}

func TestHandlePlaybackMessageCommandNotPaired(t *testing.T) {
	ctrl := &Client{
		userID:   "user4",
		deviceID: "",
		send:     make(chan []byte, 10),
	}
	msg := []byte(`{"type":"command","command":"play"}`)
	HandlePlaybackMessage(ctrl, msg)

	select {
	case raw := <-ctrl.send:
		var m PlaybackMessage
		err := json.Unmarshal(raw, &m)
		assert.NoError(t, err)
		assert.Equal(t, "error", m.Type)
	default:
		t.Fatal("expected error message")
	}
}
