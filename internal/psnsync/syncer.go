package psnsync

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strconv"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/connectors/psn"
	"github.com/knightsofeternity/kfire-server/internal/store"
	"github.com/knightsofeternity/kfire-server/internal/ws"
)

const (
	libraryEvery   = 6 * time.Hour
	trophyGamesMax = 10
	sessionSource  = "psn_api"
	provider       = "psn"
)

// PresenceFunc turns a user into the hub's presence payload. Injected from the
// api package (presenceUser), which also applies the member's chosen status.
type PresenceFunc func(ctx context.Context, userID string) (ws.PresenceUser, bool)

type Syncer struct {
	st       *store.Store
	bot      *Bot
	hub      *ws.Hub
	presence PresenceFunc
}

func New(st *store.Store, bot *Bot, hub *ws.Hub, presence PresenceFunc) *Syncer {
	return &Syncer{st: st, bot: bot, hub: hub, presence: presence}
}

// SetPresenceFunc is called by api.Register, which owns presenceUser.
func (s *Syncer) SetPresenceFunc(f PresenceFunc) { s.presence = f }

func (s *Syncer) Bot() *Bot { return s.bot }

// toAccept keeps the pending requests that come from linked members.
func toAccept(requests []string, linked map[string]string) []string {
	var out []string
	for _, id := range requests {
		if _, ok := linked[id]; ok {
			out = append(out, id)
		}
	}
	return out
}

func batches(ids []string, size int) [][]string {
	var out [][]string
	for len(ids) > size {
		out = append(out, ids[:size])
		ids = ids[size:]
	}
	if len(ids) > 0 {
		out = append(out, ids)
	}
	return out
}

// linkedMembers maps PSN account id to KFIRE user id.
func (s *Syncer) linkedMembers(ctx context.Context) (map[string]string, error) {
	users, err := s.st.ListLinkedByProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(users))
	for _, u := range users {
		m[u.ProviderUserID] = u.UserID
	}
	return m, nil
}

// Tick runs one pass: accept linked members' requests, reconcile presence,
// import due libraries.
func (s *Syncer) Tick(ctx context.Context) {
	tok, err := s.bot.Token(ctx)
	if errors.Is(err, ErrNeedsNPSSO) {
		// Blind until an admin pastes a new NPSSO: a console session left
		// open would show members "in game" for as long as that takes.
		s.closeSessions(ctx, nil)
		return
	}
	if errors.Is(err, ErrNoBot) {
		return
	}
	if err != nil {
		slog.Warn("psnsync: token", "err", err)
		return
	}
	linked, err := s.linkedMembers(ctx)
	if err != nil || len(linked) == 0 {
		return
	}
	conn := s.bot.Conn()

	if reqs, err := conn.ReceivedRequests(ctx, tok); err != nil {
		slog.Warn("psnsync: friend requests", "err", err)
	} else {
		for _, id := range toAccept(reqs, linked) {
			if err := conn.AcceptFriend(ctx, tok, id); err != nil {
				slog.Warn("psnsync: accept friend", "user_id", linked[id], "err", err)
				continue
			}
			slog.Info("psnsync: friend request accepted", "user_id", linked[id])
		}
	}

	friends, err := conn.Friends(ctx, tok)
	if err != nil {
		slog.Warn("psnsync: friends", "err", err)
		return
	}
	var ids []string
	friend := make(map[string]bool, len(friends))
	for _, f := range friends {
		if _, ok := linked[f]; ok {
			ids = append(ids, f)
			friend[f] = true
		}
	}
	// A member who removed the bot can no longer be seen: close their session
	// instead of leaving them "in game".
	s.closeSessions(ctx, func(accountID string) bool { return !friend[accountID] })
	sort.Strings(ids)
	for _, batch := range batches(ids, 100) {
		ps, err := conn.Presences(ctx, tok, batch)
		if err != nil {
			slog.Warn("psnsync: presences", "err", err)
			continue
		}
		for _, p := range ps {
			if userID, ok := linked[p.AccountID]; ok && s.reconcile(ctx, userID, p) && s.presence != nil {
				if pu, ok := s.presence(ctx, userID); ok {
					s.hub.BroadcastPresence(ctx, pu)
				}
			}
		}
	}

	for _, acc := range ids {
		userID := linked[acc]
		due, err := s.st.PsnLibraryDue(ctx, userID, libraryEvery)
		if err != nil || !due {
			continue
		}
		if err := s.SyncLibrary(ctx, userID, acc); err != nil {
			slog.Warn("psnsync: library", "user_id", userID, "err", err)
		}
	}
}

// closeSessions ends the open psn_api session of every linked member for whom
// which returns true (all of them when which is nil), broadcasting the change.
func (s *Syncer) closeSessions(ctx context.Context, which func(accountID string) bool) {
	linked, err := s.linkedMembers(ctx)
	if err != nil {
		return
	}
	for acc, userID := range linked {
		if which != nil && !which(acc) {
			continue
		}
		open, err := s.st.OpenSessionBySource(ctx, userID, sessionSource)
		if err != nil || open == nil {
			continue
		}
		if changed, _ := s.st.EndSession(ctx, userID, open.Game.ID); changed && s.presence != nil {
			if pu, ok := s.presence(ctx, userID); ok {
				s.hub.BroadcastPresence(ctx, pu)
			}
		}
	}
}

// reconcile opens, keeps or closes the member's psn_api session.
func (s *Syncer) reconcile(ctx context.Context, userID string, p psn.Presence) bool {
	open, err := s.st.OpenSessionBySource(ctx, userID, sessionSource)
	if err != nil {
		return false
	}
	if p.TitleID == "" {
		if open != nil {
			changed, _ := s.st.EndSession(ctx, userID, open.Game.ID)
			return changed
		}
		return false
	}
	game, err := s.st.PsnGameForTitle(ctx, p.TitleID, psn.NormalizeTitle(p.TitleName))
	if err != nil {
		slog.Error("psnsync: resolve game", "title", p.TitleName, "err", err)
		return false
	}
	if open != nil && open.Game.ID == game.ID {
		return false
	}
	if open != nil {
		if _, err := s.st.EndSession(ctx, userID, open.Game.ID); err != nil {
			slog.Error("psnsync: end previous session", "user_id", userID, "err", err)
			return false
		}
	}
	changed, err := s.st.StartSession(ctx, userID, game.ID, sessionSource)
	if err != nil {
		slog.Error("psnsync: start session", "user_id", userID, "err", err)
		return false
	}
	return changed
}

// SyncLibrary imports a member's played games with their hours, then the
// earned trophies of the most recently played games.
func (s *Syncer) SyncLibrary(ctx context.Context, userID, accountID string) error {
	tok, err := s.bot.Token(ctx)
	if err != nil {
		return err
	}
	conn := s.bot.Conn()
	games, err := conn.PlayedGames(ctx, tok, accountID)
	if err != nil {
		return err
	}
	// PS4 and PS5 entries of one concept add up under one game.
	seconds := map[string]int64{}
	for _, g := range games {
		ids := g.ConceptTitle
		if len(ids) == 0 {
			ids = []string{g.TitleID}
		}
		concept := g.ConceptID
		if concept == "" || concept == "<nil>" || concept == "0" {
			concept = "title:" + g.TitleID // no concept: the title stands alone
		}
		cg, err := s.st.UpsertPsnGame(ctx, concept, ids, psn.NormalizeTitle(g.Name), g.ImageURL)
		if err != nil {
			slog.Warn("psnsync: game", "name", g.Name, "err", err)
			continue
		}
		seconds[cg.ID] += g.Seconds
	}
	for gameID, secs := range seconds {
		if err := s.st.UpsertExternalPlaytime(ctx, userID, provider, gameID, secs); err != nil {
			return err
		}
	}

	titles, err := conn.TrophyTitles(ctx, tok, accountID)
	if err != nil {
		return err
	}
	sort.Slice(titles, func(i, j int) bool { return titles[i].LastUpdated.After(titles[j].LastUpdated) })
	if len(titles) > trophyGamesMax {
		titles = titles[:trophyGamesMax]
	}
	for _, t := range titles {
		trophies, err := conn.EarnedTrophies(ctx, tok, accountID, t)
		if err != nil {
			slog.Warn("psnsync: trophies", "title", t.Name, "err", err)
			continue
		}
		game, ok, err := s.st.PsnGameForTrophies(ctx, t.NpCommunicationID, psn.NormalizeTitle(t.Name))
		if err != nil || !ok {
			// No game of that name: the trophies wait until one exists rather
			// than creating a near-duplicate of a game the member does have.
			continue
		}
		rows := make([]store.AchievementRow, 0, len(trophies))
		for _, tr := range trophies {
			rows = append(rows, store.AchievementRow{
				GameID: game.ID, APIName: t.NpCommunicationID + ":" + strconv.Itoa(tr.ID),
				DisplayName: tr.Name, IconURL: tr.IconURL, UnlockedAt: tr.EarnedAt,
			})
		}
		if err := s.st.UpsertAchievements(ctx, userID, provider, rows); err != nil {
			return err
		}
	}
	return s.st.MarkPsnLibrarySynced(ctx, userID)
}

// Run polls every interval until ctx ends.
func (s *Syncer) Run(ctx context.Context, interval time.Duration) {
	slog.Info("psnsync: poller started", "interval", interval)
	t := time.NewTimer(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Tick(ctx)
			t.Reset(interval)
		}
	}
}
