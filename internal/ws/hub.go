// Package ws implements the real-time presence hub.
//
// Protocol reference: https://github.com/knightsofeternity/kfire-protocol/blob/main/websocket-events.md
package ws

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
)

// Envelope is the wire format shared by every WebSocket message.
type Envelope struct {
	Type    string          `json:"type"`
	TS      time.Time       `json:"ts"`
	Payload json.RawMessage `json:"payload"`
}

// client is one authenticated WebSocket connection.
type client struct {
	conn   *websocket.Conn
	userID string
	send   chan []byte
}

// Hub fans presence events out to every connected client of the org.
//
// TODO(mvp): back the hub with Redis pub/sub so multiple server replicas share
// presence state; for now everything is in-process.
type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
}

// NewHub creates an empty hub.
func NewHub() *Hub {
	return &Hub{clients: make(map[*client]struct{})}
}

// Broadcast sends a marshalled envelope to every connected client.
func (h *Hub) Broadcast(typ string, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		slog.Error("ws: marshal broadcast payload", "type", typ, "err", err)
		return
	}
	msg, err := json.Marshal(Envelope{Type: typ, TS: time.Now().UTC(), Payload: raw})
	if err != nil {
		slog.Error("ws: marshal broadcast envelope", "type", typ, "err", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- msg:
		default:
			// Slow consumer: drop the message rather than block the hub.
			slog.Warn("ws: dropping message for slow client", "user_id", c.userID)
		}
	}
}

// Handler returns the connection handler to mount on the /ws route.
func (h *Hub) Handler() func(*websocket.Conn) {
	return func(conn *websocket.Conn) {
		c := &client{conn: conn, send: make(chan []byte, 32)}

		// TODO(mvp): enforce the handshake from websocket-events.md —
		// require a `hello` envelope with a valid JWT within 10s, reply
		// `hello_ack`, then mark the user online and broadcast presence.
		h.register(c)
		defer h.unregister(c)

		go c.writeLoop()
		c.readLoop(h)
	}
}

func (h *Hub) register(c *client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	slog.Info("ws: client connected", "clients", h.count())
}

func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	close(c.send)
	slog.Info("ws: client disconnected", "clients", h.count())
}

func (h *Hub) count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (c *client) writeLoop() {
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

func (c *client) readLoop(h *Hub) {
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var env Envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			slog.Warn("ws: invalid envelope", "err", err)
			continue
		}

		switch env.Type {
		case "hello":
			// TODO(mvp): validate JWT + protocol_version, send hello_ack.
		case "game_started", "game_stopped":
			// TODO(mvp): persist the session and broadcast presence_update.
			slog.Info("ws: presence event (stub)", "type", env.Type)
		case "heartbeat":
			// TODO(mvp): refresh the connection liveness deadline.
		default:
			// Unknown types are ignored for forward compatibility.
		}
	}
}
