// Package psnsync runs the PlayStation connector: the bot's token, friend
// requests, presence, library and trophies.
package psnsync

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/connectors/psn"
	"github.com/knightsofeternity/kfire-server/internal/crypto"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

// ErrNoBot means no admin has pasted an NPSSO: the connector is off.
var ErrNoBot = errors.New("psn: no bot configured")

// ErrNeedsNPSSO means Sony refused the stored NPSSO: an admin must paste a new one.
var ErrNeedsNPSSO = errors.New("psn: the bot needs a new NPSSO")

type Bot struct {
	st     *store.Store
	conn   *psn.Connector
	cipher *crypto.Cipher
	mu     sync.Mutex // one token renewal at a time
}

func NewBot(st *store.Store, conn *psn.Connector, cipher *crypto.Cipher) *Bot {
	return &Bot{st: st, conn: conn, cipher: cipher}
}

// Conn exposes the connector to the syncer and the handlers.
func (b *Bot) Conn() *psn.Connector { return b.conn }

// Configured reports whether an admin ever set an NPSSO (even a revoked one):
// the PlayStation card is shown from then on.
func (b *Bot) Configured(ctx context.Context) bool {
	_, err := b.st.GetPsnBot(ctx)
	return err == nil
}

// SetNPSSO stores a new NPSSO and proves it right away: a revoked or mistyped
// one is refused here, in front of the admin, rather than found by the poller.
func (b *Bot) SetNPSSO(ctx context.Context, npsso string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	tok, err := b.conn.Exchange(ctx, npsso)
	if err != nil {
		return err
	}
	me, err := b.conn.Me(ctx, tok.Access)
	if err != nil {
		return err
	}
	enc, err := b.cipher.SealString(npsso)
	if err != nil {
		return err
	}
	if err := b.st.SetPsnNPSSO(ctx, enc); err != nil {
		return err
	}
	return b.save(ctx, tok, me)
}

// Token returns a valid access token: the stored one, else a refresh, else a
// new exchange with the NPSSO. A refused NPSSO marks the bot and returns
// ErrNeedsNPSSO; nothing is retried until an admin pastes a new one.
func (b *Bot) Token(ctx context.Context) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	bot, err := b.st.GetPsnBot(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return "", ErrNoBot
	}
	if err != nil {
		return "", err
	}
	if bot.Status == "needs_npsso" {
		return "", ErrNeedsNPSSO
	}
	if bot.AccessTokenEnc != nil && bot.AccessExpiresAt != nil && time.Until(*bot.AccessExpiresAt) > 2*time.Minute {
		return b.cipher.OpenString(bot.AccessTokenEnc)
	}
	if bot.RefreshTokenEnc != nil && bot.RefreshExpiresAt != nil && time.Until(*bot.RefreshExpiresAt) > time.Hour {
		if refresh, err := b.cipher.OpenString(bot.RefreshTokenEnc); err == nil {
			if tok, err := b.conn.Refresh(ctx, refresh); err == nil {
				return tok.Access, b.save(ctx, tok, psn.Profile{})
			}
			slog.Warn("psn: refresh failed, falling back to the NPSSO")
		}
	}
	npsso, err := b.cipher.OpenString(bot.NPSSOEnc)
	if err != nil {
		return "", err
	}
	tok, err := b.conn.Exchange(ctx, npsso)
	if errors.Is(err, psn.ErrLoginRequired) {
		slog.Error("psn: Sony refused the bot's NPSSO (login_required 4165); an admin must paste a new one",
			"npsso_set_at", bot.NPSSOSetAt)
		if merr := b.st.MarkPsnNeedsNPSSO(ctx, "Sony a refusé le jeton du bot (login_required 4165)"); merr != nil {
			slog.Error("psn: mark needs_npsso", "err", merr)
		}
		return "", ErrNeedsNPSSO
	}
	if err != nil {
		return "", err
	}
	return tok.Access, b.save(ctx, tok, psn.Profile{})
}

func (b *Bot) save(ctx context.Context, tok psn.Tokens, me psn.Profile) error {
	accessEnc, err := b.cipher.SealString(tok.Access)
	if err != nil {
		return err
	}
	refreshEnc, err := b.cipher.SealString(tok.Refresh)
	if err != nil {
		return err
	}
	return b.st.SavePsnTokens(ctx, accessEnc, tok.AccessExpires, refreshEnc, tok.RefreshExpires, me.OnlineID, me.AccountID)
}

// Status is what the admin card shows. Never the NPSSO itself.
type Status struct {
	Configured bool       `json:"configured"`
	Status     string     `json:"status"`
	OnlineID   *string    `json:"online_id"`
	LastError  *string    `json:"last_error"`
	NPSSOSetAt *time.Time `json:"npsso_set_at"`
	LastOKAt   *time.Time `json:"last_ok_at"`
}

func (b *Bot) Status(ctx context.Context) (Status, error) {
	bot, err := b.st.GetPsnBot(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return Status{}, nil
	}
	if err != nil {
		return Status{}, err
	}
	return Status{Configured: true, Status: bot.Status, OnlineID: bot.OnlineID,
		LastError: bot.LastError, NPSSOSetAt: &bot.NPSSOSetAt, LastOKAt: bot.LastOKAt}, nil
}

// OnlineID is the bot's pseudo, for the member card ("add … as a friend").
func (b *Bot) OnlineID(ctx context.Context) string {
	bot, err := b.st.GetPsnBot(ctx)
	if err != nil || bot.OnlineID == nil {
		return ""
	}
	return *bot.OnlineID
}
