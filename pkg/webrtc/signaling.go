package webrtc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog/log"

	"github.com/devuser/aetherstream/pkg/config"
)

// SignalingServer handles WebRTC SDP exchange via WebSocket
type SignalingServer struct {
	upgrader websocket.Upgrader
	peers    map[string]*PeerConnection
	mu       sync.RWMutex
}

// PeerConnection wraps a pion/webrtc peer with metadata
type PeerConnection struct {
	ID       string
	PC       *webrtc.PeerConnection
	WS       *websocket.Conn
	mu       sync.Mutex
	stopCh   chan struct{}
}

// NewSignalingServer creates a new WebRTC signaling server
func NewSignalingServer(cfg *config.Config) *SignalingServer {
	allowed := []string{"http://localhost:8080", "http://localhost:8081"}
	if cfg != nil && len(cfg.Server.AllowedOrigins) > 0 {
		allowed = cfg.Server.AllowedOrigins
	}
	return &SignalingServer{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				for _, o := range allowed {
					if strings.EqualFold(origin, o) {
						return true
					}
				}
				return false
			},
		},
		peers: make(map[string]*PeerConnection),
	}
}

// HandleNegotiate upgrades HTTP to WebSocket and handles signaling
func (s *SignalingServer) HandleNegotiate(w http.ResponseWriter, r *http.Request) {
	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("WebSocket upgrade failed")
		return
	}
	defer ws.Close()

	peerID := fmt.Sprintf("peer-%d", time.Now().UnixNano())
	log.Info().Str("peer", peerID).Msg("WebRTC peer connected")

	// Create peer connection
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}
	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		log.Error().Err(err).Msg("PeerConnection creation failed")
		return
	}
	defer pc.Close()

	peer := &PeerConnection{
		ID:     peerID,
		PC:     pc,
		WS:     ws,
		stopCh: make(chan struct{}),
	}

	s.mu.Lock()
	s.peers[peerID] = peer
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.peers, peerID)
		s.mu.Unlock()
		close(peer.stopCh)
	}()

	// Handle incoming signaling messages
	for {
		select {
		case <-peer.stopCh:
			return
		default:
		}

		msgType, data, err := ws.ReadMessage()
		if err != nil {
			log.Error().Err(err).Str("peer", peerID).Msg("WebSocket read error")
			return
		}
		if msgType != websocket.TextMessage {
			continue
		}

		var msg SignalingMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Error().Err(err).Str("peer", peerID).Msg("Invalid signaling message")
			continue
		}

		if err := s.handleSignalingMessage(peer, &msg); err != nil {
			log.Error().Err(err).Str("peer", peerID).Msg("Signaling handling failed")
		}
	}
}

// SignalingMessage represents a WebRTC signaling message
type SignalingMessage struct {
	Type string          `json:"type"` // offer, answer, ice-candidate
	SDP  string          `json:"sdp,omitempty"`
	ICE  *ICECandidate   `json:"ice,omitempty"`
}

// ICECandidate represents a parsed ICE candidate
type ICECandidate struct {
	Candidate string `json:"candidate"`
	SDPMLineIndex uint16 `json:"sdpMLineIndex"`
	SDPMid string `json:"sdpMid"`
}

func (s *SignalingServer) handleSignalingMessage(peer *PeerConnection, msg *SignalingMessage) error {
	switch msg.Type {
	case "offer":
		return s.handleOffer(peer, msg.SDP)
	case "answer":
		return s.handleAnswer(peer, msg.SDP)
	case "ice-candidate":
		return s.handleICECandidate(peer, msg.ICE)
	default:
		return fmt.Errorf("unknown signaling type: %s", msg.Type)
	}
}

func (s *SignalingServer) handleOffer(peer *PeerConnection, sdp string) error {
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sdp,
	}

	if err := peer.PC.SetRemoteDescription(offer); err != nil {
		return fmt.Errorf("set remote description: %w", err)
	}

	answer, err := peer.PC.CreateAnswer(nil)
	if err != nil {
		return fmt.Errorf("create answer: %w", err)
	}

	if err := peer.PC.SetLocalDescription(answer); err != nil {
		return fmt.Errorf("set local description: %w", err)
	}

	// Send answer back
	resp := SignalingMessage{
		Type: "answer",
		SDP:  answer.SDP,
	}
	data, _ := json.Marshal(resp)
	peer.mu.Lock()
	defer peer.mu.Unlock()
	return peer.WS.WriteMessage(websocket.TextMessage, data)
}

func (s *SignalingServer) handleAnswer(peer *PeerConnection, sdp string) error {
	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  sdp,
	}
	return peer.PC.SetRemoteDescription(answer)
}

func (s *SignalingServer) handleICECandidate(peer *PeerConnection, ice *ICECandidate) error {
	if ice == nil {
		return nil
	}
	candidate := webrtc.ICECandidateInit{
		Candidate: ice.Candidate,
		SDPMLineIndex: &ice.SDPMLineIndex,
		SDPMid: &ice.SDPMid,
	}
	return peer.PC.AddICECandidate(candidate)
}

// GetPeer returns a peer by ID
func (s *SignalingServer) GetPeer(id string) (*PeerConnection, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	peer, ok := s.peers[id]
	return peer, ok
}

// PeerCount returns the number of active peers
func (s *SignalingServer) PeerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.peers)
}
