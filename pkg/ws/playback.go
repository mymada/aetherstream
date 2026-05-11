package ws

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

// PlaybackClientType identifies the role of a WebSocket client in playback.
type PlaybackClientType string

const (
	PlaybackTypeController PlaybackClientType = "controller"
	PlaybackTypeReceiver   PlaybackClientType = "receiver"
)

// PlaybackMessage is the unified envelope for playback WebSocket messages.
type PlaybackMessage struct {
	Type      string          `json:"type"`                // "command" | "event" | "pair" | "paired" | "error"
	SessionID string          `json:"session_id,omitempty"`
	DeviceID  string          `json:"device_id,omitempty"`
	Command   string          `json:"command,omitempty"`   // play | pause | seek | stop | volume
	State     string          `json:"state,omitempty"`     // playing | paused | stopped | buffering
	Position  int64           `json:"position,omitempty"`  // ms
	Volume    int             `json:"volume,omitempty"`    // 0-100
	Payload   json.RawMessage `json:"payload,omitempty"`   // free-form extensibility
}

// PlaybackSession links a receiver (TV) with an optional controller (phone).
type PlaybackSession struct {
	DeviceID   string
	Receiver   *Client // TV / browser
	Controller *Client // phone / remote (may be nil until paired)
	mu         sync.RWMutex
}

// PlaybackHub extends Hub with playback session management.
type PlaybackHub struct {
	sessions map[string]*PlaybackSession // key = device_id
	mu       sync.RWMutex
}

var (
	playbackHub     *PlaybackHub
	playbackHubOnce sync.Once
)

func getPlaybackHub() *PlaybackHub {
	playbackHubOnce.Do(func() {
		playbackHub = &PlaybackHub{
			sessions: make(map[string]*PlaybackSession),
		}
	})
	return playbackHub
}

// GenerateDeviceID creates a unique TV device identifier.
func GenerateDeviceID() string {
	return "tv-" + uuid.New().String()
}

// RegisterReceiver creates a new PlaybackSession for a TV/receiver and sends back its device_id.
func RegisterReceiver(client *Client) string {
	ph := getPlaybackHub()
	deviceID := GenerateDeviceID()

	client.deviceID = deviceID

	session := &PlaybackSession{
		DeviceID: deviceID,
		Receiver: client,
	}

	ph.mu.Lock()
	ph.sessions[deviceID] = session
	ph.mu.Unlock()

	// Notify receiver of its device_id
	msg := PlaybackMessage{Type: "pair", DeviceID: deviceID}
	b, _ := json.Marshal(msg)
	select {
	case client.send <- b:
	default:
	}

	log.Info().Str("device_id", deviceID).Msg("playback receiver registered")
	return deviceID
}

// PairController associates a controller client with an existing receiver session.
func PairController(client *Client, deviceID string) bool {
	ph := getPlaybackHub()
	ph.mu.Lock()
	defer ph.mu.Unlock()

	session, ok := ph.sessions[deviceID]
	if !ok {
		sendError(client, "session not found")
		return false
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	// If there was an old controller, just overwrite (new phone takes over)
	if session.Controller != nil && session.Controller != client {
		log.Info().Str("device_id", deviceID).Msg("controller replaced")
	}
	session.Controller = client
	client.deviceID = deviceID

	// Notify controller it is paired
	msg := PlaybackMessage{Type: "paired", DeviceID: deviceID}
	b, _ := json.Marshal(msg)
	select {
	case client.send <- b:
	default:
	}

	log.Info().Str("device_id", deviceID).Str("user", client.userID).Msg("controller paired")
	return true
}

// UnregisterPlayback removes a client from its playback session.
// Called on disconnect.
func UnregisterPlayback(client *Client) {
	if client == nil || client.deviceID == "" {
		return
	}
	ph := getPlaybackHub()
	ph.mu.Lock()
	defer ph.mu.Unlock()

	session, ok := ph.sessions[client.deviceID]
	if !ok {
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Receiver == client {
		// Receiver disconnected → tear down whole session
		delete(ph.sessions, client.deviceID)
		log.Info().Str("device_id", client.deviceID).Msg("playback session destroyed (receiver gone)")
	} else if session.Controller == client {
		session.Controller = nil
		log.Info().Str("device_id", client.deviceID).Msg("controller disconnected")
	}
}

// ForwardCommand routes a controller command to its paired receiver.
func ForwardCommand(deviceID string, msg PlaybackMessage) bool {
	ph := getPlaybackHub()
	ph.mu.RLock()
	session, ok := ph.sessions[deviceID]
	ph.mu.RUnlock()
	if !ok {
		return false
	}

	session.mu.RLock()
	receiver := session.Receiver
	session.mu.RUnlock()

	if receiver == nil {
		return false
	}

	b, err := json.Marshal(msg)
	if err != nil {
		return false
	}

	select {
	case receiver.send <- b:
		log.Debug().Str("device_id", deviceID).Str("cmd", msg.Command).Msg("command forwarded to receiver")
		return true
	default:
		return false
	}
}

// ForwardEvent routes a receiver event to its paired controller.
func ForwardEvent(deviceID string, msg PlaybackMessage) bool {
	ph := getPlaybackHub()
	ph.mu.RLock()
	session, ok := ph.sessions[deviceID]
	ph.mu.RUnlock()
	if !ok {
		return false
	}

	session.mu.RLock()
	controller := session.Controller
	session.mu.RUnlock()

	if controller == nil {
		// No controller yet; event is dropped silently (or could be buffered)
		return false
	}

	b, err := json.Marshal(msg)
	if err != nil {
		return false
	}

	select {
	case controller.send <- b:
		log.Debug().Str("device_id", deviceID).Str("state", msg.State).Msg("event forwarded to controller")
		return true
	default:
		return false
	}
}

// HandlePlaybackMessage processes a JSON message from a playback client.
func HandlePlaybackMessage(client *Client, raw []byte) {
	var msg PlaybackMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		sendError(client, "invalid json")
		return
	}

	switch msg.Type {
	case "pair":
		// Controller wants to pair with a TV
		if msg.DeviceID == "" {
			sendError(client, "missing device_id")
			return
		}
		PairController(client, msg.DeviceID)

	case "command":
		// Must come from a controller and target its paired receiver
		if client.deviceID == "" {
			sendError(client, "not paired")
			return
		}
		ForwardCommand(client.deviceID, msg)

	case "event":
		// Must come from a receiver and target its paired controller
		if client.deviceID == "" {
			// Receiver not registered? ignore
			return
		}
		ForwardEvent(client.deviceID, msg)

	default:
		// Unknown type, ignore or log
		log.Debug().Str("type", msg.Type).Msg("unknown playback message type")
	}
}

func sendError(client *Client, text string) {
	msg := PlaybackMessage{Type: "error", Payload: json.RawMessage(`"` + text + `"`)}
	b, _ := json.Marshal(msg)
	select {
	case client.send <- b:
	default:
	}
}

// ------------------------------------------------------------------
// WebSocket entry points for playback clients
// ------------------------------------------------------------------

// HandlePlaybackWebSocket is the generic playback upgrade handler.
// The clientType query param decides if this client is a "receiver" or "controller".
func HandlePlaybackWebSocket(conn *websocket.Conn, clientType PlaybackClientType, userID string) {
	client := &Client{
		conn:   conn,
		userID: userID,
		send:   make(chan []byte, 256),
	}

	// Register in global hub so broadcast works
	hub := getGlobalHub()
	hub.mu.Lock()
	key := userID + "-" + client.deviceID
	if client.deviceID == "" {
		key = userID + "-playback-" + uuid.New().String()
	}
	hub.clients[key] = client
	hub.mu.Unlock()

	defer func() {
		hub.mu.Lock()
		delete(hub.clients, key)
		hub.mu.Unlock()

		UnregisterPlayback(client)
		close(client.send)
		_ = conn.Close()
	}()

	// If receiver, auto-register and get device_id
	if clientType == PlaybackTypeReceiver {
		RegisterReceiver(client)
		// update key after deviceID is known
		hub.mu.Lock()
		delete(hub.clients, key)
		key = userID + "-" + client.deviceID
		hub.clients[key] = client
		hub.mu.Unlock()
	}

	// Writer goroutine
	go client.writePump()

	// Reader loop
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Warn().Err(err).Str("user", userID).Str("type", string(clientType)).Msg("playback websocket error")
			}
			break
		}
		HandlePlaybackMessage(client, raw)
	}
}
