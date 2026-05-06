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
type InboundHandler func(userID, deviceID string, msg InboundMessage)

// OutboundHandler is called for every non-empty message sent to a client.
type OutboundHandler func(eventType string, payload any) any

// Hub manages connected WebSocket clients and broadcasts messages.
type Hub struct {
	// mu controls concurrent access to the clients map.
	// Multiple goroutines interact with the map in http handlers, so ensure
	// proper locking during read/writes.
	mu sync.RWMutex
	// clients contains all active WebSocket clients.
	clients map[*client]struct{}
	// log is the logger for the hub.
	log *slog.Logger
	// onMessage is the callback for inbound client messages.
	onMessage InboundHandler
	// transform is the callback for outbound client messages.
	transform OutboundHandler
}

// messageReadLimit is the maximum size of a message (in bytes) that can be read from the WebSocket.
const messageReadLimit = 65536

// New creates and returns a new Hub.
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

// SetOutboundPayloadTransformer registers a callback that can compact/transform
// outbound payloads per event type before they are marshaled and sent.
func (h *Hub) SetOutboundPayloadTransformer(transform OutboundHandler) {
	h.transform = transform
}

// transformedPayload returns the payload transformed by the outbound handler.
// It is called before the payload is marshaled and sent to clients.
func (h *Hub) transformedPayload(eventType string, payload any) any {
	if h.transform == nil {
		return payload
	}

	return h.transform(eventType, payload)
}

// Publish sends an event to every connected client.
func (h *Hub) Publish(eventType string, payload any) {
	env := Envelope{Type: eventType, Payload: h.transformedPayload(eventType, payload)}
	data, err := json.Marshal(env)
	if err != nil {
		h.log.Error("ws marshal", "type", eventType, "err", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- data:
		default:
			h.log.Warn("client send buffer full, dropping message", "userID", c.userID, "deviceID", c.deviceID, "eventType", eventType)
		}
	}
}

// PublishToUser sends an event only to clients authenticated as the given user.
func (h *Hub) PublishToUser(userID, eventType string, payload any) {
	env := Envelope{Type: eventType, Payload: h.transformedPayload(eventType, payload)}
	data, err := json.Marshal(env)
	if err != nil {
		h.log.Error("ws marshal", "type", eventType, "err", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if c.userID == userID {
			select {
			case c.send <- data:
			default:
				h.log.Warn("client send buffer full, dropping message", "userID", c.userID, "deviceID", c.deviceID, "eventType", eventType)
			}
		}
	}
}

// PublishToDevice sends an event only to a specific device of a user.
func (h *Hub) PublishToDevice(userID, deviceID, eventType string, payload any) {
	env := Envelope{Type: eventType, Payload: h.transformedPayload(eventType, payload)}

	data, err := json.Marshal(env)
	if err != nil {
		h.log.Error("ws marshal", "type", eventType, "err", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.clients {
		if c.userID == userID && c.deviceID == deviceID {
			select {
			case c.send <- data:
			default:
				h.log.Warn("client send buffer full, dropping message", "userID", c.userID, "deviceID", c.deviceID, "eventType", eventType)
			}
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
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, userID, deviceID string) {
	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		h.log.Error("ws upgrade", "err", err)
		return
	}

	c := &client{
		hub:      h,
		conn:     conn,
		send:     make(chan []byte, 64),
		userID:   userID,
		deviceID: deviceID,
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
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	userID   string
	deviceID string
}

// readPump pumps messages from the WebSocket connection to the hub.
func (c *client) readPump() {
	defer c.hub.unregister(c)

	// defines the period of inactivity allowed before a connection is closed.
	deadline := 60 * time.Second

	c.conn.SetReadLimit(messageReadLimit)
	c.conn.SetReadDeadline(time.Now().Add(deadline))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(deadline))
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
				// isolate panics in the message handler to prevent crashing the entire hub and affecting other clients
				func() {
					defer func() {
						if r := recover(); r != nil {
							c.hub.log.Error("panic in ws handler", "type", msg.Type, "recover", r)
						}
					}()
					c.hub.onMessage(c.userID, c.deviceID, msg)
				}()
			}
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection.
func (c *client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	deadline := 10 * time.Second

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(deadline))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(deadline))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
