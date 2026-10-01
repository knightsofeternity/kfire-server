// Package nintendosync runs the Nintendo Switch connector: the bot's login and
// session, friend requests, presence and play history.
package nintendosync

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/connectors/nintendo"
	"github.com/knightsofeternity/kfire-server/internal/crypto"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

var (
	// ErrNoBot means no admin logged a bot in: the connector is off.
	ErrNoBot = errors.New("nintendo: no bot configured")
	// ErrNeedsLogin means Nintendo refused the bot's session: an admin must
	// log it in again.
	ErrNeedsLogin = errors.New("nintendo: the bot must be logged in again")
	// ErrLoginExpired is a pasted link for no login in progress, or too old.
	ErrLoginExpired = errors.New("nintendo: no login in progress, generate a new link")
)

// loginTTL bounds how long a generated login link stays usable.
const loginTTL = 15 * time.Minute

type Bot struct {
	st       *store.Store
	client   *nintendo.Client
	accounts *nintendo.Accounts
	cipher   *crypto.Cipher
	mu       sync.Mutex
}

func NewBot(st *store.Store, client *nintendo.Client, accounts *nintendo.Accounts, cipher *crypto.Cipher) *Bot {
	return &Bot{st: st, client: client, accounts: accounts, cipher: cipher}
}

func (b *Bot) Client() *nintendo.Client { return b.client }

// Enabled reports whether the instance has a sidecar at all.
func (b *Bot) Enabled() bool { return b.client.Enabled() }

// Configured reports whether a bot has ever been logged in: the member card
// is shown from then on.
func (b *Bot) Configured(ctx context.Context) bool {
	if !b.Enabled() {
		return false
	}
	bot, err := b.st.GetNintendoBot(ctx)
	return err == nil && bot.SessionEnc != nil
}

// StartLogin prepares the Nintendo app's login flow and returns the link the
// admin opens. The PKCE verifier is kept sealed until the link comes back.
func (b *Bot) StartLogin(ctx context.Context) (string, error) {
	l := b.accounts.NewLogin()
	enc, err := b.cipher.SealString(l.Verifier)
	if err != nil {
		return "", err
	}
	if err := b.st.StartNintendoLogin(ctx, l.State, enc); err != nil {
		return "", err
	}
	return l.URL, nil
}

// FinishLogin takes the npf link the admin copied, turns it into a session
// token and proves it through the sidecar before storing anything.
func (b *Bot) FinishLogin(ctx context.Context, link string) error {
	code, state, err := nintendo.ParseRedirect(link)
	if err != nil {
		return err
	}
	bot, err := b.st.GetNintendoBot(ctx)
	if err != nil || bot.LoginState == nil || *bot.LoginState != state ||
		bot.LoginStartedAt == nil || time.Since(*bot.LoginStartedAt) > loginTTL {
		return ErrLoginExpired
	}
	verifier, err := b.cipher.OpenString(bot.LoginVerifierEnc)
	if err != nil {
		return err
	}
	session, err := b.accounts.Exchange(ctx, code, verifier)
	if err != nil {
		return err
	}
	return b.SetSession(ctx, session)
}

// SetSession proves a session token (KFIRE_NINTENDO_SESSION_TOKEN seeds it on
// first boot) and stores it with the bot's identity.
func (b *Bot) SetSession(ctx context.Context, session string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	me, err := b.client.Me(ctx, session)
	if err != nil {
		return err
	}
	enc, err := b.cipher.SealString(session)
	if err != nil {
		return err
	}
	return b.st.SetNintendoSession(ctx, enc, me.Name, me.NsaID, me.FriendCode)
}

// Session returns the bot's session token, or why there is none.
func (b *Bot) Session(ctx context.Context) (string, error) {
	if !b.Enabled() {
		return "", ErrNoBot
	}
	bot, err := b.st.GetNintendoBot(ctx)
	if errors.Is(err, store.ErrNotFound) || (err == nil && bot.SessionEnc == nil) {
		return "", ErrNoBot
	}
	if err != nil {
		return "", err
	}
	if bot.Status == "needs_login" {
		return "", ErrNeedsLogin
	}
	return b.cipher.OpenString(bot.SessionEnc)
}

// Revoked records that Nintendo refused the session. Logged once, here; the
// poller then stays quiet until an admin logs the bot in again.
func (b *Bot) Revoked(ctx context.Context) {
	slog.Error("nintendo: Nintendo refused the bot's session token; an admin must log it in again (Admin > Nintendo)")
	if err := b.st.MarkNintendoNeedsLogin(ctx, "Nintendo a refusé la session du bot"); err != nil {
		slog.Error("nintendo: mark needs_login", "err", err)
	}
}

// Status is what the admin card shows. Never a token.
type Status struct {
	Enabled      bool       `json:"enabled"`
	Configured   bool       `json:"configured"`
	Status       string     `json:"status,omitempty"`
	Nickname     *string    `json:"nickname,omitempty"`
	FriendCode   *string    `json:"friend_code,omitempty"`
	LastError    *string    `json:"last_error,omitempty"`
	SessionSetAt *time.Time `json:"session_set_at,omitempty"`
	LastOKAt     *time.Time `json:"last_ok_at,omitempty"`
}

func (b *Bot) Status(ctx context.Context) (Status, error) {
	s := Status{Enabled: b.Enabled()}
	bot, err := b.st.GetNintendoBot(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	s.Configured = bot.SessionEnc != nil
	s.Status, s.Nickname, s.FriendCode = bot.Status, bot.Nickname, bot.FriendCode
	s.LastError, s.SessionSetAt, s.LastOKAt = bot.LastError, bot.SessionSetAt, bot.LastOKAt
	return s, nil
}

// Identity is the bot's name and friend code, for the member card.
func (b *Bot) Identity(ctx context.Context) (nickname, friendCode string) {
	bot, err := b.st.GetNintendoBot(ctx)
	if err != nil {
		return "", ""
	}
	if bot.Nickname != nil {
		nickname = *bot.Nickname
	}
	if bot.FriendCode != nil {
		friendCode = *bot.FriendCode
	}
	return nickname, friendCode
}
