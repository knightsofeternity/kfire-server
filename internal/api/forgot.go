package api

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/mail"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

// forgotWindow is how long after an email a member cannot trigger another.
const forgotWindow = 15 * time.Minute

// forgotStore is what the forgotten-password flow needs from the store.
type forgotStore interface {
	GetUserByLogin(ctx context.Context, login string) (store.User, error)
	CreatePasswordReset(ctx context.Context, userID string, tokenHash []byte, expiresAt time.Time) error
}

// forgotService sends a member the same reset link an admin would generate.
type forgotService struct {
	st        forgotStore
	sender    mail.Sender
	publicURL string
	org       string
	now       func() time.Time

	mu   sync.Mutex
	last map[string]time.Time // user id -> last email sent
}

func newForgotService(st forgotStore, sender mail.Sender, publicURL, org string) *forgotService {
	return &forgotService{
		st: st, sender: sender, publicURL: publicURL, org: org,
		now: time.Now, last: map[string]time.Time{},
	}
}

// claim reports whether an email may go to this member now, and records it.
// Expired entries are purged on the way: the map never outgrows the members
// who asked in the last quarter of an hour.
func (f *forgotService) claim(userID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := f.now()
	for id, at := range f.last {
		if now.Sub(at) >= forgotWindow {
			delete(f.last, id)
		}
	}
	if _, recent := f.last[userID]; recent {
		return false
	}
	f.last[userID] = now
	return true
}

// handle does the whole job and says how it ended, for the log. The caller
// answers the visitor the same way whatever this returns.
func (f *forgotService) handle(ctx context.Context, login, lang string) string {
	u, err := f.st.GetUserByLogin(ctx, login)
	if errors.Is(err, store.ErrNotFound) {
		return "no_account"
	}
	if err != nil {
		return "error"
	}
	if u.BannedAt != nil {
		return "banned"
	}
	if u.Email == "" {
		return "no_email"
	}
	if !f.claim(u.ID) {
		return "throttled"
	}
	token, err := randResetToken()
	if err != nil {
		return "error"
	}
	if err := f.st.CreatePasswordReset(ctx, u.ID, hashResetToken(token), f.now().Add(passwordResetTTL)); err != nil {
		return "error"
	}
	m := resetEmail(lang, f.org, u.Username, f.publicURL+"/reset/"+token)
	m.To, m.ToName = u.Email, u.Username
	if err := f.sender.Send(ctx, m); err != nil {
		return "send_failed: " + err.Error()
	}
	return "sent"
}

// resetEmail builds the message in French or English; anything else is English.
// The username is escaped in the HTML part only.
func resetEmail(lang, org, username, link string) mail.Message {
	u := html.EscapeString(username)
	o := html.EscapeString(org)
	l := html.EscapeString(link)
	if lang == "fr" {
		return mail.Message{
			Subject: fmt.Sprintf("Réinitialisation de votre mot de passe %s", org),
			Text: fmt.Sprintf("Bonjour %s,\n\nPour choisir un nouveau mot de passe %s, ouvrez ce lien :\n%s\n\n"+
				"Il est valable 1 heure et ne sert qu'une fois.\n\n"+
				"Si vous n'avez rien demandé, ignorez ce message : votre mot de passe ne change pas.\n",
				username, org, link),
			HTML: fmt.Sprintf("<p>Bonjour %s,</p><p>Pour choisir un nouveau mot de passe %s, ouvrez ce lien :</p>"+
				"<p><a href=\"%s\">%s</a></p><p>Il est valable 1 heure et ne sert qu'une fois.</p>"+
				"<p>Si vous n'avez rien demandé, ignorez ce message : votre mot de passe ne change pas.</p>",
				u, o, l, l),
		}
	}
	return mail.Message{
		Subject: fmt.Sprintf("Reset your %s password", org),
		Text: fmt.Sprintf("Hello %s,\n\nTo choose a new %s password, open this link:\n%s\n\n"+
			"It is valid for 1 hour and works only once.\n\n"+
			"If you did not ask for this, ignore this message: your password does not change.\n",
			username, org, link),
		HTML: fmt.Sprintf("<p>Hello %s,</p><p>To choose a new %s password, open this link:</p>"+
			"<p><a href=\"%s\">%s</a></p><p>It is valid for 1 hour and works only once.</p>"+
			"<p>If you did not ask for this, ignore this message: your password does not change.</p>",
			u, o, l, l),
	}
}

// POST /api/v1/auth/forgot  (public)
//
// Always answers 202 the same way, account or not, and does the work after
// answering: a known account must not answer slower than an unknown one.
func (h *handlers) forgotPassword(c *fiber.Ctx) error {
	if h.forgot == nil {
		return errorJSON(c, fiber.StatusNotFound, "not_available", "password reset by email is not configured")
	}
	var req struct {
		Login string `json:"login"`
		Lang  string `json:"lang"`
	}
	if err := c.BodyParser(&req); err != nil {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed", "invalid JSON body")
	}
	login := strings.TrimSpace(req.Login)
	if login == "" {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed", "login is required")
	}
	go func(login, lang string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// Neither the login nor the address is logged: only how it ended.
		slog.Info("forgot: done", "outcome", h.forgot.handle(ctx, login, lang))
	}(login, req.Lang)
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"status": "sent_if_exists"})
}
