package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// upgrader upgrades the HTTP connection to WebSocket.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Envelope is the JSON message sent over the WebSocket (outbound).
type Envelope struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

// InboundMessage is a JSON envelope received from a WebSocket client.
type InboundMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// InboundHandler is called for every non-empty message received from a client.
// userID is the authenticated user extracted at connection time.
type InboundHandler func(userID string, msg InboundMessage)

// Hub manages connected WebSocket clients and broadcasts messages.
type Hub struct {
	// mu controls concurrent access to the clients map.
	// Multiple goroutines interact with the map in http handlers, so ensure
	// proper locking during read/writes.
	mu sync.RWMutex

	clients   map[*client]struct{}
	log       *slog.Logger
	onMessage InboundHandler
}

func New() *Hub {
	return &Hub{
		clients: make(map[*client]struct{}),
		log:     slog.Default().With("component", "ws"),
	}
}

// SetMessageHandler registers a callback for inbound client messages.
func (h *Hub) SetMessageHandler(handler InboundHandler) {
	h.onMessage = handler
}

// Publish sends an event to every connected client.
func (h *Hub) Publish(eventType string, payload any) {
	env := Envelope{Type: eventType, Payload: payload}
	data := mustMarshal(env)

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- data:
		default:
		}
	}
}

// ConnectedCount returns the number of active WebSocket clients.
func (h *Hub) ConnectedCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ServeWS upgrades the HTTP connection to WebSocket.
// userID should be the authenticated user's ID (empty string if unauthenticated).
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, userID string) {
	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		h.log.Error("ws upgrade", "err", err)
		return
	}

	c := &client{
		hub:    h,
		conn:   conn,
		send:   make(chan []byte, 64),
		userID: userID,
	}

	h.register(c)
	go c.writePump()
	go c.readPump()
}

// register adds a client to the hub.
func (h *Hub) register(c *client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

// unregister removes a client from the hub.
func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	close(c.send)
	c.conn.Close()
}

// client is a WebSocket client.
type client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	userID string
}

// readPump pumps messages from the WebSocket connection to the hub.
func (c *client) readPump() {
	defer c.hub.unregister(c)

	c.conn.SetReadLimit(4096)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		if c.hub.onMessage != nil {
			var msg InboundMessage
			if json.Unmarshal(data, &msg) == nil && msg.Type != "" {
				c.hub.onMessage(c.userID, msg)
			}
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection.
func (c *client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// mustMarshal is a wrapper function around json.Marshal(...) that panics if an error occurs.
func mustMarshal(v any) []byte {
	data, err := json.Marshal(v)

	if err != nil {
		panic("ws marshal: " + err.Error())
	}

	return data
}
