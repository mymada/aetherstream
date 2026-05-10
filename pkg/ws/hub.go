package ws

import (
	"net/http"
	"sync"
	"time"

	"github.com/devuser/aetherstream/pkg/db"
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
	clients map[string]*Client // key = userID
	mu      sync.RWMutex
}

type Client struct {
	conn     *websocket.Conn
	userID   string
	deviceID string
	send     chan []byte
}

var globalHub = &Hub{
	clients: make(map[string]*Client),
}

// HandleWebSocket upgrades HTTP to WebSocket
func HandleWebSocket(c echo.Context, database *db.DB) error {
	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		log.Warn().Err(err).Msg("websocket upgrade failed")
		return err
	}

	userID := c.QueryParam("user_id")
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

	globalHub.mu.Lock()
	globalHub.clients[userID+"-"+deviceID] = client
	globalHub.mu.Unlock()

	// Cleanup on disconnect
	defer func() {
		globalHub.mu.Lock()
		delete(globalHub.clients, userID+"-"+deviceID)
		globalHub.mu.Unlock()
		conn.Close()
	}()

	// Send welcome
	client.send <- []byte(`{"type":"connected","server":"AetherStream"}`)

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
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.WriteMessage(websocket.TextMessage, message)

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
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
	globalHub.mu.RLock()
	defer globalHub.mu.RUnlock()
	for _, client := range globalHub.clients {
		select {
		case client.send <- msg:
		default:
			// Channel full, skip
		}
	}
}

// BroadcastToUser sends to all devices of a specific user
func BroadcastToUser(userID string, msg []byte) {
	globalHub.mu.RLock()
	defer globalHub.mu.RUnlock()
	for key, client := range globalHub.clients {
		if len(key) > len(userID) && key[:len(userID)] == userID {
			select {
			case client.send <- msg:
			default:
			}
		}
	}
}
