// Package epicsync links members' Epic Games accounts and imports their Epic
// library and the playtime Epic counts, every 6 hours and on demand. No
// presence: Epic does not expose what a member is playing (probe 2026-10-04),
// and the desktop client already detects Epic games running on the PC.
package epicsync

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/connectors/epic"
	"github.com/knightsofeternity/kfire-server/internal/crypto"
	"github.com/knightsofeternity/kfire-server/internal/gametitle"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

var (
	// ErrNoCode is a paste with no authorizationCode in it.
	ErrNoCode = errors.New("epicsync: no authorization code in the pasted text")
	// ErrTaken is an Epic account already linked to another member.
	ErrTaken = errors.New("epicsync: Epic account linked to another member")
	// ErrNeedsRelink is a member whose refresh token Epic refused.
	ErrNeedsRelink = errors.New("epicsync: the member must link Epic again")
)

const provider = "epic"

type Syncer struct {
	st     *store.Store
	conn   *epic.Connector
	cipher *crypto.Cipher
}

func New(st *store.Store, conn *epic.Connector, cipher *crypto.Cipher) *Syncer {
	return &Syncer{st: st, conn: conn, cipher: cipher}
}

func (s *Syncer) Conn() *epic.Connector { return s.conn }

// IsGame keeps games and drops add-on content, which Epic files as "hidden".
func IsGame(categories []string) bool {
	return slices.Contains(categories, "games") && !slices.Contains(categories, "hidden")
}

var (
	// testBuild spots the test clients a library also lists ("Chivalry 2 -
	// Public Testing", "KillingFloor2Beta"): they are not games of their own.
	testBuild = regexp.MustCompile(`(?i)(public\s*test(ing)?|play\s*test|\bPTS\b|\btest\s*server\b|\bbeta\b|beta$)`)
	// editionTail is a store edition appended to a game's name.
	editionTail = regexp.MustCompile(`(?i)\s*[-:\x{2013}\x{2014}]?\s*(?:Game\s+of\s+the\s+Year|GOTY|Complete|Definitive|Deluxe|Ultimate|Gold|Standard|Premium|Anniversary)\s+Edition\s*$`)
)

// IsTestBuild reports a test or beta client of a game.
func IsTestBuild(title, appName string) bool {
	return testBuild.MatchString(title) || testBuild.MatchString(appName)
}

// withoutEdition drops a store edition from a normalized name.
func withoutEdition(name string) string {
	return strings.TrimSpace(editionTail.ReplaceAllString(name, ""))
}

// gameName is the name an Epic title joins the catalog under: the full name
// when a catalog game carries it ("RollerCoaster Tycoon 3: Complete Edition"
// exists as such), else the name without its edition ("Dragon Age:
// Inquisition – Game of the Year Edition" joins "Dragon Age: Inquisition").
func (s *Syncer) gameName(ctx context.Context, title string) (string, error) {
	full := gametitle.Normalize(title)
	short := withoutEdition(full)
	if short == "" || short == full {
		return full, nil
	}
	ok, err := s.st.GameNamed(ctx, full)
	if err != nil || ok {
		return full, err
	}
	return short, nil
}

// secondsByGame gives every owned game its Epic playtime (0 when Epic counts
// none, so it still shows in the library). Artifacts that are not owned games
// (add-ons) are ignored, or one session would count several times.
func secondsByGame(games map[string]string, play []epic.Playtime) map[string]int64 {
	out := make(map[string]int64, len(games))
	for _, gameID := range games {
		out[gameID] += 0
	}
	for _, p := range play {
		if gameID, ok := games[p.ArtifactID]; ok {
			out[gameID] += p.TotalTime
		}
	}
	return out
}

// Link exchanges the pasted code and stores the member's sealed token.
func (s *Syncer) Link(ctx context.Context, userID, pasted string) (string, error) {
	code, ok := epic.ParseCode(pasted)
	if !ok {
		return "", ErrNoCode
	}
	tok, err := s.conn.Exchange(ctx, code)
	if err != nil {
		return "", err
	}
	taken, err := s.st.ProviderLinkedToOther(ctx, provider, tok.AccountID, userID)
	if err != nil {
		return "", err
	}
	if taken {
		return "", ErrTaken
	}
	enc, err := s.cipher.SealString(tok.Refresh)
	if err != nil {
		return "", err
	}
	if err := s.st.SaveEpicLink(ctx, userID, tok.AccountID, tok.DisplayName, enc, tok.RefreshExpires); err != nil {
		return "", err
	}
	return tok.DisplayName, nil
}

// SyncUser imports one member's library and playtime.
func (s *Syncer) SyncUser(ctx context.Context, userID string) error {
	enc, err := s.st.EpicRefreshToken(ctx, userID)
	if err != nil {
		return err
	}
	refresh, err := s.cipher.OpenString(enc)
	if err != nil {
		return err
	}
	tok, err := s.conn.Refresh(ctx, refresh)
	if errors.Is(err, epic.ErrInvalidGrant) {
		if err := s.st.MarkEpicNeedsRelink(ctx, userID); err != nil {
			return err
		}
		return ErrNeedsRelink
	}
	if err != nil {
		return err
	}
	newEnc, err := s.cipher.SealString(tok.Refresh)
	if err != nil {
		return err
	}
	if err := s.st.UpdateEpicToken(ctx, userID, newEnc, tok.RefreshExpires); err != nil {
		return err
	}

	items, err := s.conn.Library(ctx, tok.Access)
	if err != nil {
		return err
	}
	games := map[string]string{} // appName → KFIRE game id
	for _, it := range items {
		t, known, err := s.st.EpicTitleByKey(ctx, it.Namespace, it.CatalogItemID)
		if err != nil {
			return err
		}
		if !known {
			cat, err := s.conn.CatalogItem(ctx, tok.Access, it.Namespace, it.CatalogItemID)
			if err != nil {
				slog.Warn("epicsync: catalog item", "namespace", it.Namespace, "item", it.CatalogItemID, "err", err)
				continue
			}
			t = store.EpicTitle{Namespace: it.Namespace, CatalogItemID: it.CatalogItemID, AppName: it.AppName,
				Title: cat.Title, IsGame: IsGame(cat.Categories) && !IsTestBuild(cat.Title, it.AppName), ImageURL: cat.Image}
			if t.IsGame {
				name, err := s.gameName(ctx, cat.Title)
				if err != nil {
					return err
				}
				g, err := s.st.UpsertEpicGame(ctx, name, cat.Image)
				if err != nil {
					return err
				}
				t.GameID = &g.ID
			}
			if err := s.st.SaveEpicTitle(ctx, t); err != nil {
				return err
			}
		}
		if t.IsGame && t.GameID != nil {
			games[it.AppName] = *t.GameID
		}
	}

	play, err := s.conn.Playtime(ctx, tok.Access, tok.AccountID)
	if err != nil {
		return err
	}
	for gameID, secs := range secondsByGame(games, play) {
		if err := s.st.UpsertExternalPlaytime(ctx, userID, provider, gameID, secs); err != nil {
			return err
		}
	}
	return s.st.MarkEpicSynced(ctx, userID)
}

// SyncAll syncs every live link. A refused launcher client stops the pass:
// every member would fail the same way until the admin fixes the .env.
func (s *Syncer) SyncAll(ctx context.Context) {
	ids, err := s.st.EpicLinkedOK(ctx)
	if err != nil {
		slog.Error("epicsync: list links", "err", err)
		return
	}
	for _, id := range ids {
		err := s.SyncUser(ctx, id)
		switch {
		case err == nil:
			slog.Info("epicsync: user synced", "user_id", id)
		case errors.Is(err, epic.ErrClientRejected):
			slog.Error("epic: launcher client rejected, check KFIRE_EPIC_CLIENT_ID/SECRET")
			return
		case errors.Is(err, ErrNeedsRelink):
			slog.Info("epicsync: token refused, member must link again", "user_id", id)
		default:
			slog.Warn("epicsync: user sync failed", "user_id", id, "err", err)
		}
	}
}

// Run syncs everyone shortly after boot, then on the interval.
func (s *Syncer) Run(ctx context.Context, interval time.Duration) {
	slog.Info("epicsync: poller started", "interval", interval)
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			s.SyncAll(ctx)
			timer.Reset(interval)
		}
	}
}
