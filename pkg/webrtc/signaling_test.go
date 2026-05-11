package webrtc

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSignalingServer(t *testing.T) {
	srv := NewSignalingServer(nil)
	assert.NotNil(t, srv)
	assert.Equal(t, 0, srv.PeerCount())
}

func TestPeerConnectionLifecycle(t *testing.T) {
	srv := NewSignalingServer(nil)

	// Simulate peer creation (without actual WebSocket)
	peer := &PeerConnection{
		ID:     "test-peer-1",
		stopCh: make(chan struct{}),
	}

	srv.mu.Lock()
	srv.peers["test-peer-1"] = peer
	srv.mu.Unlock()

	assert.Equal(t, 1, srv.PeerCount())

	retrieved, ok := srv.GetPeer("test-peer-1")
	assert.True(t, ok)
	assert.Equal(t, "test-peer-1", retrieved.ID)

	// Simulate disconnect
	close(peer.stopCh)
	srv.mu.Lock()
	delete(srv.peers, "test-peer-1")
	srv.mu.Unlock()

	assert.Equal(t, 0, srv.PeerCount())
}

func TestSignalingMessageParsing(t *testing.T) {
	msg := SignalingMessage{
		Type: "offer",
		SDP:  "v=0\r\n...",
	}
	assert.Equal(t, "offer", msg.Type)
	assert.NotEmpty(t, msg.SDP)
}

func TestICECandidate(t *testing.T) {
	ice := &ICECandidate{
		Candidate:     "candidate:1 1 UDP 2130706431 192.168.1.1 5000 typ host",
		SDPMLineIndex: 0,
		SDPMid:        "0",
	}
	assert.NotEmpty(t, ice.Candidate)
	assert.Equal(t, uint16(0), ice.SDPMLineIndex)
}

func TestSignalingServerConcurrency(t *testing.T) {
	srv := NewSignalingServer(nil)

	// Add multiple peers concurrently
	for i := 0; i < 10; i++ {
		go func(id int) {
			peer := &PeerConnection{
				ID:     fmt.Sprintf("peer-%d", id),
				stopCh: make(chan struct{}),
			}
			srv.mu.Lock()
			srv.peers[peer.ID] = peer
			srv.mu.Unlock()
		}(i)
	}

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 10, srv.PeerCount())
}
