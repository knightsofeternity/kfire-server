// Package gogsync links members' GOG accounts and imports their GOG library
// and the playtime GOG Galaxy counted, every 6 hours and on demand. No
// presence: GOG games run on the PC, where the desktop client already detects
// them.
package gogsync

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/connectors/gog"
	"github.com/knightsofeternity/kfire-server/internal/crypto"
	"github.com/knightsofeternity/kfire-server/internal/gametitle"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

var (
	// ErrNoCode is a paste with no code in it.
	ErrNoCode = errors.New("gogsync: no code in the pasted text")
	// ErrTaken is a GOG account already linked to another member.
	ErrTaken = errors.New("gogsync: GOG account linked to another member")
	// ErrNeedsRelink is a member whose refresh token GOG refused.
	ErrNeedsRelink = errors.New("gogsync: the member must link GOG again")
)

const provider = "gog"

type Syncer struct {
	st     *store.Store
	conn   *gog.Connector
	cipher *crypto.Cipher
}

func New(st *store.Store, conn *gog.Connector, cipher *crypto.Cipher) *Syncer {
	return &Syncer{st: st, conn: conn, cipher: cipher}
}

func (s *Syncer) Conn() *gog.Connector { return s.conn }

// IsGame keeps games; DLC and packs ("pack" bundles several games, each also
// listed on its own) are not.
func IsGame(gameType string) bool { return gameType == "game" }

// gameName is the name a GOG title joins the catalog under: the full name
// when a catalog game carries it, else the name without its edition ("The
// Witcher 3: Wild Hunt - Game of the Year Edition" joins "The Witcher 3: Wild
// Hunt").
func (s *Syncer) gameName(ctx context.Context, title string) (string, error) {
	full := gametitle.Normalize(title)
	short := gametitle.WithoutEdition(full)
	if short == "" || short == full {
		return full, nil
	}
	ok, err := s.st.GameNamed(ctx, full)
	if err != nil || ok {
		return full, err
	}
	return short, nil
}

// Link exchanges the pasted code and stores the member's sealed token.
func (s *Syncer) Link(ctx context.Context, userID, pasted string) (string, error) {
	code, ok := gog.ParseCode(pasted)
	if !ok {
		return "", ErrNoCode
	}
	tok, err := s.conn.Exchange(ctx, code)
	if err != nil {
		return "", err
	}
	taken, err := s.st.ProviderLinkedToOther(ctx, provider, tok.UserID, userID)
	if err != nil {
		return "", err
	}
	if taken {
		return "", ErrTaken
	}
	name, err := s.conn.Username(ctx, tok.Access)
	if err != nil {
		// The name is only shown on the account card: not worth failing the link.
		slog.Warn("gogsync: username", "user_id", userID, "err", err)
	}
	enc, err := s.cipher.SealString(tok.Refresh)
	if err != nil {
		return "", err
	}
	if err := s.st.SaveGogLink(ctx, userID, tok.UserID, name, enc); err != nil {
		return "", err
	}
	return name, nil
}

// SyncUser imports one member's GOG games and the minutes Galaxy counted.
func (s *Syncer) SyncUser(ctx context.Context, userID string) error {
	acc, err := s.st.GogAccountFor(ctx, userID)
	if err != nil {
		return err
	}
	enc, err := s.st.GogRefreshToken(ctx, userID)
	if err != nil {
		return err
	}
	refresh, err := s.cipher.OpenString(enc)
	if err != nil {
		return err
	}
	tok, err := s.conn.Refresh(ctx, refresh)
	if errors.Is(err, gog.ErrInvalidGrant) {
		if err := s.st.MarkGogNeedsRelink(ctx, userID); err != nil {
			return err
		}
		return ErrNeedsRelink
	}
	if err != nil {
		return err
	}
	if tok.Refresh != "" && tok.Refresh != refresh {
		newEnc, err := s.cipher.SealString(tok.Refresh)
		if err != nil {
			return err
		}
		if err := s.st.UpdateGogToken(ctx, userID, newEnc); err != nil {
			return err
		}
	}

	ids, err := s.conn.OwnedGOGProducts(ctx, tok.Access, acc.GalaxyUserID)
	if err != nil {
		return err
	}
	if err := s.resolveTitles(ctx, ids); err != nil {
		return err
	}
	for _, id := range ids {
		t, known, err := s.st.GogTitleByID(ctx, id)
		if err != nil {
			return err
		}
		if !known || !t.IsGame || t.GameID == nil {
			continue
		}
		minutes, err := s.conn.PlayedMinutes(ctx, tok.Access, acc.GalaxyUserID, id)
		if err != nil {
			return err
		}
		// An owned game without hours still gets its row (0), so it shows in
		// the library.
		if err := s.st.UpsertExternalPlaytime(ctx, userID, provider, *t.GameID, minutes*60); err != nil {
			return err
		}
	}
	return s.st.MarkGogSynced(ctx, userID)
}

// resolveTitles describes the products not cached yet, in one catalog pass.
func (s *Syncer) resolveTitles(ctx context.Context, ids []string) error {
	var unknown []string
	for _, id := range ids {
		_, known, err := s.st.GogTitleByID(ctx, id)
		if err != nil {
			return err
		}
		if !known {
			unknown = append(unknown, id)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	prods, err := s.conn.Products(ctx, unknown)
	if err != nil {
		return err
	}
	for _, p := range prods {
		t := store.GogTitle{ProductID: p.ID, Title: p.Title, IsGame: IsGame(p.GameType), ImageURL: p.Image}
		if t.IsGame {
			name, err := s.gameName(ctx, p.Title)
			if err != nil {
				return err
			}
			g, err := s.st.UpsertGogGame(ctx, name, p.Image)
			if err != nil {
				return err
			}
			t.GameID = &g.ID
		}
		if err := s.st.SaveGogTitle(ctx, t); err != nil {
			return err
		}
	}
	return nil
}

// SyncAll syncs every live link. A refused Galaxy client stops the pass:
// every member would fail the same way until the admin fixes the .env.
func (s *Syncer) SyncAll(ctx context.Context) {
	ids, err := s.st.GogLinkedOK(ctx)
	if err != nil {
		slog.Error("gogsync: list links", "err", err)
		return
	}
	for _, id := range ids {
		err := s.SyncUser(ctx, id)
		switch {
		case err == nil:
			slog.Info("gogsync: user synced", "user_id", id)
		case errors.Is(err, gog.ErrClientRejected):
			slog.Error("gog: Galaxy client rejected, check KFIRE_GOG_CLIENT_ID/SECRET")
			return
		case errors.Is(err, ErrNeedsRelink):
			slog.Info("gogsync: token refused, member must link again", "user_id", id)
		default:
			slog.Warn("gogsync: user sync failed", "user_id", id, "err", err)
		}
	}
}

// Run syncs everyone shortly after boot, then on the interval.
func (s *Syncer) Run(ctx context.Context, interval time.Duration) {
	slog.Info("gogsync: poller started", "interval", interval)
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
