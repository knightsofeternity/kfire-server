// Package ws implements the real-time presence hub.
//
// Protocol reference: https://github.com/knightsofeternity/kfire-protocol/blob/main/websocket-events.md
package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/contrib/websocket"

	"github.com/knightsofeternity/kfire-server/internal/auth"
	"github.com/knightsofeternity/kfire-server/internal/matchrecord"
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

// matchEnvelope is everything the hub needs to understand about a match
// result: which game, and the rest as-is. The shape of the rest belongs to
// the game, not to the control plane.
//
// Identical to gameEventPayload today, and yet distinct on purpose: a game
// event is a presence state the hub interprets in full, while this is only
// the label of a body the hub will never read. Merging them would suggest
// they evolve together.
type matchEnvelope struct {
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
	// presenceStatus is the user's chosen status (online/invisible/offline),
	// cached at hello so connect/disconnect broadcasts honor an invisible/offline
	// member (otherwise they would pop back online on every reconnect).
	presenceStatus string
}

// PresenceUser is the minimal identity the hub needs to build a presence
// entry. The API layer passes it when a privacy toggle must take effect live.
type PresenceUser struct {
	ID              string
	Username        string
	AvatarURL       *string
	ActivityVisible bool
	PresenceStatus  string
}

func (c *client) presenceUser() PresenceUser {
	return PresenceUser{
		ID:              c.userID,
		Username:        c.username,
		AvatarURL:       c.avatarURL,
		ActivityVisible: c.activityVisible,
		PresenceStatus:  c.presenceStatus,
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
	publicURL string
	recorders *matchrecord.Registry
	mu        sync.RWMutex
	clients   map[*client]struct{}
	online    map[string]*onlineState // by user ID
	live      map[string]liveEntry    // in-progress match state, by member ID
	// liveVisible says, per member, whether their live match may be
	// broadcast.
	//
	// This state lives in the hub, under h.mu, and NOT on the connection,
	// unlike the equivalent fields read at hello time. That is deliberate:
	// the invisibility toggle arrives over an HTTP request, so from a
	// different goroutine than the connection's read loop. Writing the
	// connection's fields there would be a race, caught by the race
	// detector. Here, the lock already protecting the rest of the shared
	// state takes care of it.
	liveVisible map[string]bool
}

// NewHub creates an empty hub. jwtSecret verifies the access tokens presented
// in `hello` handshakes; st persists sessions and resolves games; publicURL
// builds image-proxy URLs in presence broadcasts; recorders routes a match
// result to the game that knows how to read it.
func NewHub(jwtSecret []byte, st *store.Store, publicURL string, recorders *matchrecord.Registry) *Hub {
	return &Hub{
		jwtSecret:   jwtSecret,
		store:       st,
		publicURL:   publicURL,
		recorders:   recorders,
		clients:     make(map[*client]struct{}),
		online:      make(map[string]*onlineState),
		live:        make(map[string]liveEntry),
		liveVisible: make(map[string]bool),
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
	hadLive := false
	h.mu.Lock()
	delete(h.clients, c)
	wasLastConn := false
	if c.authenticated.Load() {
		if st, ok := h.online[c.userID]; ok {
			st.conns--
			if st.conns <= 0 {
				delete(h.online, c.userID)
				wasLastConn = true
				hadLive = h.live[c.userID].payload.GameSlug != ""
				delete(h.live, c.userID)
				delete(h.liveVisible, c.userID)
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

	// The last connection dropping takes the in-progress match with it:
	// without this, a member who closes their client mid-match would stay
	// displayed as "in match" until the server restarts.
	if hadLive {
		h.Broadcast("live_match", map[string]any{"user_id": c.userID, "match": nil})
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
// An open session (any source) counts as in_game when visible, so console
// players with no WebSocket connection appear live. A hidden open session does
// not reveal the game and only yields online when WS-connected.
func (h *Hub) BroadcastPresence(ctx context.Context, u PresenceUser) {
	online := h.OnlineSince(u.ID)
	var sess *store.Session
	if s, err := h.store.LatestOpenSession(ctx, u.ID); err == nil {
		sess = s
	} else {
		slog.Error("ws: latest open session", "user_id", u.ID, "err", err)
	}
	status := store.PresenceStatus(sess != nil, sess != nil && u.ActivityVisible, online != nil)
	// A chosen invisible/offline status forces offline for all viewers, dropping
	// the game/since fields below since they gate on in_game/online.
	status = store.ApplyPresenceOverride(u.PresenceStatus, status)

	entry := map[string]any{"user_id": u.ID, "username": u.Username, "status": status, "game": nil}
	if u.AvatarURL != nil {
		entry["avatar_url"] = *u.AvatarURL
	}
	if status == "in_game" && sess != nil {
		entry["since"] = sess.StartedAt
		entry["game"] = h.gameJSON(sess.Game)
	} else if status == "online" && online != nil {
		entry["since"] = online
	}
	h.Broadcast("presence_update", entry)
}

func (h *Hub) gameJSON(g store.Game) map[string]any {
	m := map[string]any{"id": g.ID, "name": g.Name, "slug": g.Slug}
	if g.IconURL != nil {
		m["icon_url"] = h.publicURL + "/img/games/" + g.ID + "/icon"
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
		case "match_result":
			c.handleMatchResult(h, env)
		case "live_match":
			c.handleLiveMatch(h, env)
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
	c.presenceStatus = u.PresenceStatus
	h.setLiveVisible(u.ID, u.ActivityVisible, u.PresenceStatus)
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

// handleMatchResult records one finished match reported by the client.
//
// Unlike a game event, this changes no presence and broadcasts nothing: a
// match is history, not a state.
func (c *client) handleMatchResult(h *Hub, env Envelope) {
	// The payload is decoded TWICE, here for the slug alone and then in the
	// recorder for the game's fields. This is intentional: passing along a
	// partial decode would give the hub back the knowledge of the game it
	// was just relieved of, to save a few microseconds on one message per
	// match.
	var e matchEnvelope
	if err := json.Unmarshal(env.Payload, &e); err != nil || e.GameSlug == "" {
		c.sendError("invalid_match", "malformed match result payload", false)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	game, err := h.store.GetGameBySlug(ctx, e.GameSlug)
	if err != nil {
		c.sendError("unknown_game", "game slug not in the catalog: "+e.GameSlug, false)
		return
	}

	err = h.recorders.Record(ctx, e.GameSlug, c.userID, game.ID, env.Payload)
	switch {
	case err == nil:
		slog.Info("ws: match result", "user_id", c.userID, "slug", game.Slug)
	case errors.Is(err, matchrecord.ErrUnknownGame):
		// The game is in the catalog but no recorder tracks it: this is a
		// client ahead of this server, not a write error.
		c.sendError("unknown_game", "this server does not track matches for "+e.GameSlug, false)
	case errors.Is(err, matchrecord.ErrInvalidPayload):
		// The client only gets a generic code; it has no use for our
		// internal rules. The log, though, carries the exact reason:
		// without it, a client whose serialization is broken and
		// legitimate data that hit a rule written too early look alike,
		// and neither is diagnosable.
		slog.Warn("ws: match result rejected", "user_id", c.userID, "slug", game.Slug, "err", err)
		c.sendError("invalid_match", "malformed match result payload", false)
	default:
		slog.Error("ws: persist match result", "user_id", c.userID, "slug", game.Slug, "err", err)
		// Tell the client, otherwise it drops the match from its queue
		// believing it landed.
		c.sendError("match_not_recorded", "could not record match result", false)
	}
}

// handleLiveMatch rebroadcasts the current state of a match.
//
// Nothing is written: this state lives in memory for the duration of the
// match and then disappears. It is the exact counterpart of presence, which
// is broadcast and never archived.
func (c *client) handleLiveMatch(h *Hub, env Envelope) {
	var p livePayload
	if err := json.Unmarshal(env.Payload, &p); err != nil || !p.valid() {
		c.sendError("invalid_live_match", "malformed live match state", false)
		return
	}

	// An invisible or chosen-offline member does not broadcast their match.
	// The decision is read from the hub and not from the connection,
	// because it can change through the API while the connection lives on.
	if !h.liveAllowed(c.userID) {
		return
	}

	h.setLive(c.userID, p)
	h.Broadcast("live_match", h.liveJSON(c.userID))
}

// setLive stores or clears a member's match state.
func (h *Hub) setLive(userID string, p livePayload) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if p.Ended {
		delete(h.live, userID)
		return
	}
	h.live[userID] = liveEntry{payload: p, updatedAt: time.Now()}
}

// setLiveVisible stores what a member allows for their live match.
func (h *Hub) setLiveVisible(userID string, activityVisible bool, presenceStatus string) bool {
	allowed := activityVisible &&
		store.ApplyPresenceOverride(presenceStatus, "in_game") == "in_game"
	h.mu.Lock()
	h.liveVisible[userID] = allowed
	h.mu.Unlock()
	return allowed
}

// liveAllowed reports whether this member's live match may be broadcast.
func (h *Hub) liveAllowed(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.liveVisible[userID]
}

// SetVisibility takes note of a visibility change arriving through the API,
// and immediately cuts the live match if the member just went hidden.
//
// Without this, a member who goes invisible mid-match would keep being
// broadcast to the whole guild, twice a second, until their next reconnect.
// Someone who asks not to be seen anymore must stop being seen right away.
func (h *Hub) SetVisibility(userID string, activityVisible bool, presenceStatus string) {
	if h.setLiveVisible(userID, activityVisible, presenceStatus) {
		return
	}
	h.mu.Lock()
	_, hadMatch := h.live[userID]
	delete(h.live, userID)
	h.mu.Unlock()

	// Outside the lock: Broadcast takes h.mu for reading, and an RWMutex is
	// not reentrant.
	if hadMatch {
		h.Broadcast("live_match", map[string]any{"user_id": userID, "match": nil})
	}
}

// LiveMatch returns a member's current match state, or nil if there is none
// or it expired. Exported so the REST API can serve the state to a page
// opened mid-match, which missed the earlier broadcasts.
func (h *Hub) LiveMatch(userID string) map[string]any {
	h.mu.RLock()
	e, ok := h.live[userID]
	h.mu.RUnlock()
	if !ok || e.expired(time.Now()) {
		return nil
	}
	return liveEntryJSON(e.payload)
}

// liveJSON builds the payload broadcast for a member: the state, or nil
// when the match is over.
func (h *Hub) liveJSON(userID string) map[string]any {
	return map[string]any{"user_id": userID, "match": h.LiveMatch(userID)}
}

// SweepLive clears match states that no sample has refreshed since liveTTL,
// and announces their end.
//
// Without this sweep, a member whose game crashes while KFIRE stays
// connected would leave a frozen score on the whole guild's screen forever:
// the socket does not close, so unregister never runs. LiveMatch's lazy
// expiry is not enough, since it warns no one.
func (h *Hub) SweepLive(ctx context.Context) {
	t := time.NewTicker(liveTTL / 3)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			h.mu.Lock()
			var ended []string
			for id, e := range h.live {
				if e.expired(now) {
					ended = append(ended, id)
					delete(h.live, id)
				}
			}
			h.mu.Unlock()
			// Outside the lock: Broadcast takes h.mu for reading, and an
			// RWMutex is not reentrant.
			for _, id := range ended {
				h.Broadcast("live_match", map[string]any{"user_id": id, "match": nil})
			}
		}
	}
}

// liveEntryJSON is the shape sent to the browser.
func liveEntryJSON(p livePayload) map[string]any {
	return map[string]any{
		"game_slug":         p.GameSlug,
		"team_blue_score":   p.TeamBlueScore,
		"team_orange_score": p.TeamOrangeScore,
		"seconds_remaining": p.SecondsRemaining,
		"overtime":          p.Overtime,
		"goals":             p.Goals,
		"assists":           p.Assists,
		"saves":             p.Saves,
		"shots":             p.Shots,
		"score":             p.Score,
		"demos":             p.Demos,
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
