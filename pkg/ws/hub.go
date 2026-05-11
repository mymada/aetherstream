package ws

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/devuser/aetherstream/pkg/auth"
	"github.com/devuser/aetherstream/pkg/db"
	"github.com/devuser/aetherstream/pkg/playback"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Hub manages all WebSocket connections
type Hub struct {
	clients map[string]*Client // key = userID-deviceID
	mu      sync.RWMutex
}

type Client struct {
	conn     *websocket.Conn
	userID   string
	deviceID string
	send     chan []byte
}

var (
	globalHub     *Hub
	globalHubOnce sync.Once
)

func getGlobalHub() *Hub {
	globalHubOnce.Do(func() {
		globalHub = &Hub{
			clients: make(map[string]*Client),
		}
	})
	return globalHub
}

// HandleWebSocket upgrades HTTP to WebSocket
func HandleWebSocket(c echo.Context, database *db.DB) error {
	// Validate BEFORE upgrade (fixes H3)
	if c.QueryParam("token") != "" {
		return echo.NewHTTPError(http.StatusBadRequest, "token must not be passed via query parameters")
	}

	claims := auth.GetUser(c)
	if claims == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	// Now upgrade
	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		log.Warn().Err(err).Msg("websocket upgrade failed")
		return err
	}

	userID := claims.UserID
	if userID == "" {
		userID = "anonymous"
	}
	deviceID := c.QueryParam("device_id")
	if deviceID == "" {
		deviceID = "unknown"
	}

	client := &Client{
		conn:     conn,
		userID:   userID,
		deviceID: deviceID,
		send:     make(chan []byte, 256),
	}

	hub := getGlobalHub()
	hub.mu.Lock()
	hub.clients[userID+"-"+deviceID] = client
	hub.mu.Unlock()

	// Cleanup on disconnect: close send channel, remove from hub, close conn
	defer func() {
		hub.mu.Lock()
		delete(hub.clients, userID+"-"+deviceID)
		hub.mu.Unlock()
		close(client.send)
		_ = conn.Close()
	}()

	// Send welcome
	select {
	case client.send <- []byte(`{"type":"connected","server":"AetherStream"}`):
	default:
	}

	// Writer goroutine
	go client.writePump()

	// Reader loop
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Warn().Err(err).Str("user", userID).Msg("websocket error")
			}
			break
		}
		// Handle incoming messages (playback progress, heartbeats)
		go handleMessage(client, msg, database)
	}

	return nil
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			_ = c.conn.WriteMessage(websocket.TextMessage, message)

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func handleMessage(client *Client, msg []byte, database *db.DB) {
	// Minimal: just log. Can parse JSON for playback progress.
	log.Debug().Str("user", client.userID).RawJSON("msg", msg).Msg("ws message")
}

// Broadcast sends a message to all connected clients
func Broadcast(msg []byte) {
	hub := getGlobalHub()
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	for _, client := range hub.clients {
		select {
		case client.send <- msg:
		default:
			// Channel full, skip
		}
	}
}

// BroadcastToUser sends to all devices of a specific user
func BroadcastToUser(userID string, msg []byte) {
	hub := getGlobalHub()
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	for key, client := range hub.clients {
		if len(key) > len(userID) && key[:len(userID)] == userID {
			select {
			case client.send <- msg:
			default:
			}
		}
	}
}

// HandlePlaybackWebSocketHTTP is the HTTP entry point for playback WebSocket.
// Receivers (TVs) connect without auth. Controllers (phones) must provide a valid token.
func HandlePlaybackWebSocketHTTP(c echo.Context, authSvc *auth.Service) error {
	clientType := c.QueryParam("type")
	if clientType == "" {
		clientType = string(PlaybackTypeReceiver)
	}

	var userID string
	if clientType == string(PlaybackTypeController) {
		// Controller must be authenticated
		claims := auth.GetUser(c)
		if claims == nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
		}
		userID = claims.UserID
	} else {
		// Receiver (TV) — anonymous or generate a temp ID
		userID = "tv-" + c.QueryParam("device_id")
		if userID == "tv-" {
			userID = "tv-anonymous"
		}
	}

	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		log.Warn().Err(err).Msg("playback websocket upgrade failed")
		return err
	}

	HandlePlaybackWebSocket(conn, PlaybackClientType(clientType), userID)
	return nil
}

// ForwardPlaybackEvent forwards a playback event to the WebSocket hub.
func ForwardPlaybackEvent(event playback.PlaybackEvent) {
	msg := PlaybackMessage{
		Type:      "event",
		SessionID: event.SessionID,
		Command:   string(event.Command),
		Position:  event.Position,
		Volume:    event.Volume,
	}
	b, _ := json.Marshal(msg)
	Broadcast(b)
}
