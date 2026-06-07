// Package ws implements the real-time presence hub.
//
// Protocol reference: https://github.com/knightsofeternity/kfire-protocol/blob/main/websocket-events.md
package ws

import (
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/contrib/websocket"

	"github.com/knightsofeternity/kfire-server/internal/auth"
)

const (
	// helloTimeout is how long a connection may stay unauthenticated.
	helloTimeout = 10 * time.Second
	// livenessTimeout closes connections silent for too long (heartbeat
	// is expected every 30s; 90s = 3 missed beats).
	livenessTimeout = 90 * time.Second
	// protocolVersion is the only protocol revision this server speaks.
	protocolVersion = 1
)

// Close codes from websocket-events.md.
const (
	closeAuthFailed          = 4001
	closeUnsupportedProtocol = 4002
)

// Envelope is the wire format shared by every WebSocket message.
type Envelope struct {
	Type    string          `json:"type"`
	TS      time.Time       `json:"ts"`
	Payload json.RawMessage `json:"payload"`
}

type helloPayload struct {
	ProtocolVersion int    `json:"protocol_version"`
	AccessToken     string `json:"access_token"`
	DeviceID        string `json:"device_id"`
	Client          string `json:"client"`
}

// client is one WebSocket connection.
type client struct {
	conn          *websocket.Conn
	send          chan []byte
	authenticated atomic.Bool
	userID        string
}

// Hub fans presence events out to every connected client of the org.
//
// TODO(mvp): back the hub with Redis pub/sub so multiple server replicas share
// presence state; for now everything is in-process.
type Hub struct {
	jwtSecret []byte
	mu        sync.RWMutex
	clients   map[*client]struct{}
}

// NewHub creates an empty hub. jwtSecret verifies the access tokens presented
// in `hello` handshakes.
func NewHub(jwtSecret []byte) *Hub {
	return &Hub{jwtSecret: jwtSecret, clients: make(map[*client]struct{})}
}

// Broadcast sends an envelope to every authenticated client.
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
		if !c.authenticated.Load() {
			continue
		}
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
}

func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	close(c.send)
	if c.authenticated.Load() {
		slog.Info("ws: client disconnected", "user_id", c.userID)
		// TODO(mvp): mark the user offline, close their open client-sourced
		// sessions, and broadcast a presence_update.
	}
}

func (c *client) writeLoop() {
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

func (c *client) readLoop(h *Hub) {
	// The first message must be a valid `hello` within helloTimeout.
	_ = c.conn.SetReadDeadline(time.Now().Add(helloTimeout))

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

		if !c.authenticated.Load() {
			c.handleHello(h, env)
			continue
		}

		// Any message proves liveness.
		_ = c.conn.SetReadDeadline(time.Now().Add(livenessTimeout))

		switch env.Type {
		case "game_started", "game_stopped":
			// TODO(mvp): persist the session and broadcast presence_update.
			slog.Info("ws: presence event (stub)", "type", env.Type, "user_id", c.userID)
		case "heartbeat":
			// Deadline already refreshed above.
		default:
			// Unknown types are ignored for forward compatibility.
		}
	}
}

// handleHello authenticates the connection or closes it.
func (c *client) handleHello(h *Hub, env Envelope) {
	if env.Type != "hello" {
		c.closeWithError(closeAuthFailed, "auth_failed", "first message must be hello")
		return
	}

	var p helloPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.closeWithError(closeAuthFailed, "auth_failed", "malformed hello payload")
		return
	}
	if p.ProtocolVersion != protocolVersion {
		c.closeWithError(closeUnsupportedProtocol, "unsupported_protocol",
			"this server only speaks protocol version 1")
		return
	}

	claims, err := auth.ParseAccessToken(h.jwtSecret, p.AccessToken)
	if err != nil {
		c.closeWithError(closeAuthFailed, "auth_failed", "access token invalid or expired")
		return
	}

	c.userID = claims.UserID
	c.authenticated.Store(true)
	_ = c.conn.SetReadDeadline(time.Now().Add(livenessTimeout))

	c.sendEnvelope("hello_ack", map[string]any{
		"protocol_version":           protocolVersion,
		"heartbeat_interval_seconds": 30,
		// TODO(mvp): true when an open session survived a reconnect.
		"session_resumed": false,
	})
	slog.Info("ws: client authenticated", "user_id", c.userID, "client", p.Client)
	// TODO(mvp): mark the user online and broadcast a presence_update.
}

// sendEnvelope queues a typed message for this client.
func (c *client) sendEnvelope(typ string, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		slog.Error("ws: marshal payload", "type", typ, "err", err)
		return
	}
	msg, err := json.Marshal(Envelope{Type: typ, TS: time.Now().UTC(), Payload: raw})
	if err != nil {
		return
	}
	select {
	case c.send <- msg:
	default:
	}
}

// closeWithError sends a fatal protocol error then closes the connection with
// the given close code.
func (c *client) closeWithError(closeCode int, code, message string) {
	payload, _ := json.Marshal(map[string]any{"code": code, "message": message, "fatal": true})
	msg, _ := json.Marshal(Envelope{Type: "error", TS: time.Now().UTC(), Payload: payload})
	_ = c.conn.WriteMessage(websocket.TextMessage, msg)
	_ = c.conn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(closeCode, message), time.Now().Add(time.Second))
	_ = c.conn.Close()
}
