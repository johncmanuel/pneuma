package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Envelope is the JSON message sent over the WebSocket.
type Envelope struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

// Hub manages connected WebSocket clients and broadcasts messages.
type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
	log     *slog.Logger
}

// New creates a Hub. Exported name used by app.go / cmd/server.
func New() *Hub {
	return &Hub{
		clients: make(map[*client]struct{}),
		log:     slog.Default().With("component", "ws"),
	}
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

// ServeWS is an HTTP handler that upgrades the connection to WebSocket.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error("ws upgrade", "err", err)
		return
	}
	c := &client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 64),
	}
	h.register(c)
	go c.writePump()
	go c.readPump()
}

func (h *Hub) register(c *client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	close(c.send)
	c.conn.Close()
}

type client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

func (c *client) readPump() {
	defer c.hub.unregister(c)
	c.conn.SetReadLimit(4096)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
	}
}

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

func mustMarshal(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic("ws marshal: " + err.Error())
	}
	return data
}
