package api

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

func (h *handlers) xboxRedirectURI() string {
	return h.cfg.PublicURL + "/api/v1/connect/xbox/callback"
}

// GET /api/v1/connect/xbox  (authenticated)
//
// Returns the OpenXBL sign-in URL, and sets a signed, short-lived cookie
// identifying the linking user (OpenXBL redirects back to a fixed URL without
// echoing state).
func (h *handlers) connectXboxStart(c *fiber.Ctx) error {
	if h.xbox == nil || !h.xbox.Enabled() {
		return errorJSON(c, fiber.StatusNotImplemented, "connector_disabled",
			"the Xbox connector is not configured on this instance")
	}
	state := signState([]byte(h.cfg.JWTSecret), mustClaims(c).UserID)
	c.Cookie(&fiber.Cookie{
		Name: "xbox_link", Value: state, Path: "/",
		HTTPOnly: true, Secure: true, SameSite: "Lax", MaxAge: 600,
	})
	return c.JSON(fiber.Map{"url": h.xbox.AuthURL(h.cfg.XblAppKey)})
}

// GET /api/v1/connect/xbox/callback  (public - browser redirect from OpenXBL)
func (h *handlers) connectXboxCallback(c *fiber.Ctx) error {
	if h.xbox == nil || !h.xbox.Enabled() {
		return c.Redirect("/account?xbox=error")
	}
	userID, ok := verifyState([]byte(h.cfg.JWTSecret), c.Cookies("xbox_link"))
	c.Cookie(&fiber.Cookie{Name: "xbox_link", Value: "", Path: "/", MaxAge: -1}) // clear
	if !ok {
		return c.Redirect("/account?xbox=expired")
	}
	code := c.Query("code")
	if code == "" {
		return c.Redirect("/account?xbox=denied")
	}
	memberKey, err := h.xbox.ExchangeCode(c.UserContext(), code, h.cfg.XblAppKey)
	if err != nil {
		slog.Warn("xbox claim failed", "err", err)
		return c.Redirect("/account?xbox=denied")
	}
	acc, err := h.xbox.Account(c.UserContext(), memberKey)
	if err != nil || acc.XUID == "" {
		return c.Redirect("/account?xbox=error")
	}
	taken, err := h.store.ProviderLinkedToOther(c.Context(), "xbox", acc.XUID, userID)
	if err != nil {
		return err
	}
	if taken {
		return c.Redirect("/account?xbox=conflict")
	}
	account := store.LinkedAccount{Provider: "xbox", ProviderUserID: acc.XUID}
	if acc.Gamertag != "" {
		account.DisplayName = strPtr(acc.Gamertag)
	}
	if h.cipher != nil {
		if enc, err := h.cipher.SealString(memberKey); err == nil {
			account.AccessTokenEnc = enc
		}
	}
	if err := h.store.UpsertLinkedAccount(c.Context(), userID, account); err != nil {
		return err
	}
	return c.Redirect("/account?xbox=linked")
}

// DELETE /api/v1/connect/xbox  (authenticated)
func (h *handlers) disconnectXbox(c *fiber.Ctx) error {
	err := h.store.DeleteLinkedAccount(c.Context(), mustClaims(c).UserID, "xbox")
	if errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "not_linked", "no Xbox account linked")
	}
	if err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
