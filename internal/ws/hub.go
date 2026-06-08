// Package ws implements the real-time presence hub.
//
// Protocol reference: https://github.com/knightsofeternity/kfire-protocol/blob/main/websocket-events.md
package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/contrib/websocket"

	"github.com/knightsofeternity/kfire-server/internal/auth"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

const (
	// helloTimeout is how long a connection may stay unauthenticated.
	helloTimeout = 10 * time.Second
	// livenessTimeout closes connections silent for too long (heartbeat
	// is expected every 30s; 90s = 3 missed beats).
	livenessTimeout = 90 * time.Second
	// dbTimeout bounds the store calls made from connection handlers.
	dbTimeout = 5 * time.Second
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

type gameEventPayload struct {
	GameSlug string `json:"game_slug"`
}

// client is one WebSocket connection.
type client struct {
	conn          *websocket.Conn
	send          chan []byte
	authenticated atomic.Bool
	userID        string
	username      string
	avatarURL     *string
	// activityVisible is cached from the hello handshake. Toggling it via
	// the REST API rebroadcasts presence itself, so a stale value here only
	// affects the next game event, which is acceptable.
	activityVisible bool
}

// PresenceUser is the minimal identity the hub needs to build a presence
// entry. The API layer passes it when a privacy toggle must take effect live.
type PresenceUser struct {
	ID              string
	Username        string
	AvatarURL       *string
	ActivityVisible bool
}

func (c *client) presenceUser() PresenceUser {
	return PresenceUser{
		ID:              c.userID,
		Username:        c.username,
		AvatarURL:       c.avatarURL,
		ActivityVisible: c.activityVisible,
	}
}

// onlineState tracks a connected user (potentially several connections).
type onlineState struct {
	conns    int
	since    time.Time
	username string
}

// Hub fans presence events out to every connected client of the org and
// keeps the in-memory online state.
//
// TODO(mvp): back the hub with Redis pub/sub so multiple server replicas
// share presence state; for now everything is in-process.
type Hub struct {
	jwtSecret []byte
	store     *store.Store
	mu        sync.RWMutex
	clients   map[*client]struct{}
	online    map[string]*onlineState // by user ID
}

// NewHub creates an empty hub. jwtSecret verifies the access tokens presented
// in `hello` handshakes; st persists sessions and resolves games.
func NewHub(jwtSecret []byte, st *store.Store) *Hub {
	return &Hub{
		jwtSecret: jwtSecret,
		store:     st,
		clients:   make(map[*client]struct{}),
		online:    make(map[string]*onlineState),
	}
}

// OnlineSince returns the connection time of an online user, or nil.
func (h *Hub) OnlineSince(userID string) *time.Time {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if st, ok := h.online[userID]; ok {
		t := st.since
		return &t
	}
	return nil
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
	wasLastConn := false
	if c.authenticated.Load() {
		if st, ok := h.online[c.userID]; ok {
			st.conns--
			if st.conns <= 0 {
				delete(h.online, c.userID)
				wasLastConn = true
			}
		}
	}
	h.mu.Unlock()
	close(c.send)

	if !c.authenticated.Load() {
		return
	}
	slog.Info("ws: client disconnected", "user_id", c.userID)

	if wasLastConn {
		// The user is gone: their locally-detected games are over.
		ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
		defer cancel()
		if n, err := h.store.EndClientSessions(ctx, c.userID); err != nil {
			slog.Error("ws: end client sessions", "user_id", c.userID, "err", err)
		} else if n > 0 {
			slog.Info("ws: closed open sessions on disconnect", "user_id", c.userID, "count", n)
		}
		h.BroadcastPresence(ctx, c.presenceUser())
	}
}

// connect marks an authenticated user online. Reports whether this is their
// first concurrent connection.
func (h *Hub) connect(c *client) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	st, ok := h.online[c.userID]
	if !ok {
		h.online[c.userID] = &onlineState{conns: 1, since: time.Now().UTC(), username: c.username}
		return true
	}
	st.conns++
	return false
}

// BroadcastPresence recomputes a user's presence and broadcasts it to the org.
// When the user disabled activity visibility, the game is hidden and the
// status is capped at "online".
func (h *Hub) BroadcastPresence(ctx context.Context, u PresenceUser) {
	entry := map[string]any{
		"user_id":  u.ID,
		"username": u.Username,
		"status":   "offline",
		"game":     nil,
	}
	if u.AvatarURL != nil {
		entry["avatar_url"] = *u.AvatarURL
	}

	if since := h.OnlineSince(u.ID); since != nil {
		entry["status"] = "online"
		entry["since"] = since

		if u.ActivityVisible {
			sess, err := h.store.LatestOpenSession(ctx, u.ID)
			if err != nil {
				slog.Error("ws: latest open session", "user_id", u.ID, "err", err)
			} else if sess != nil {
				entry["status"] = "in_game"
				entry["since"] = sess.StartedAt
				entry["game"] = gameJSON(sess.Game)
			}
		}
	}

	h.Broadcast("presence_update", entry)
}

func gameJSON(g store.Game) map[string]any {
	m := map[string]any{"id": g.ID, "name": g.Name, "slug": g.Slug}
	if g.IconURL != nil {
		m["icon_url"] = *g.IconURL
	}
	return m
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
		case "game_started":
			c.handleGameEvent(h, env, true)
		case "game_stopped":
			c.handleGameEvent(h, env, false)
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

	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	u, err := h.store.GetUserByID(ctx, claims.UserID)
	if err != nil || u.BannedAt != nil {
		c.closeWithError(closeAuthFailed, "auth_failed", "account unavailable")
		return
	}

	c.userID = u.ID
	c.username = u.Username
	c.avatarURL = u.AvatarURL
	c.activityVisible = u.ActivityVisible
	c.authenticated.Store(true)
	firstConn := h.connect(c)
	_ = c.conn.SetReadDeadline(time.Now().Add(livenessTimeout))

	// A session survived a brief disconnect when the user is already in game.
	sess, err := h.store.LatestOpenSession(ctx, u.ID)
	if err != nil {
		slog.Error("ws: latest open session", "user_id", u.ID, "err", err)
	}

	c.sendEnvelope("hello_ack", map[string]any{
		"protocol_version":           protocolVersion,
		"heartbeat_interval_seconds": 30,
		"session_resumed":            sess != nil,
	})
	slog.Info("ws: client authenticated", "user_id", u.ID, "username", u.Username, "client", p.Client)

	if firstConn {
		h.BroadcastPresence(ctx, c.presenceUser())
	}
}

// handleGameEvent persists a game_started/game_stopped event and broadcasts
// the resulting presence.
func (c *client) handleGameEvent(h *Hub, env Envelope, started bool) {
	var p gameEventPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil || p.GameSlug == "" {
		c.sendError("unknown_game", "missing game_slug", false)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	game, err := h.store.GetGameBySlug(ctx, p.GameSlug)
	if err != nil {
		c.sendError("unknown_game", "game slug not in the catalog: "+p.GameSlug, false)
		return
	}

	var changed bool
	if started {
		changed, err = h.store.StartSession(ctx, c.userID, game.ID, "client")
	} else {
		changed, err = h.store.EndSession(ctx, c.userID, game.ID)
	}
	if err != nil {
		slog.Error("ws: persist game event", "user_id", c.userID, "slug", p.GameSlug, "err", err)
		return
	}

	if changed {
		slog.Info("ws: game event", "user_id", c.userID, "username", c.username,
			"slug", game.Slug, "started", started)
		h.BroadcastPresence(ctx, c.presenceUser())
	}
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

// sendError sends a non-fatal protocol error notice.
func (c *client) sendError(code, message string, fatal bool) {
	c.sendEnvelope("error", map[string]any{"code": code, "message": message, "fatal": fatal})
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
