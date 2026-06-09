package api

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

func (h *handlers) battlenetRedirectURI() string {
	return h.cfg.PublicURL + "/api/v1/connect/battlenet/callback"
}

// GET /api/v1/connect/battlenet  (authenticated)
//
// Returns the Battle.net OAuth2 authorization URL for the SPA to navigate to.
func (h *handlers) connectBattlenetStart(c *fiber.Ctx) error {
	if h.battlenet == nil || !h.battlenet.Enabled() {
		return errorJSON(c, fiber.StatusNotImplemented, "connector_disabled",
			"the Battle.net connector is not configured on this instance")
	}
	state := signState([]byte(h.cfg.JWTSecret), mustClaims(c).UserID)
	return c.JSON(fiber.Map{"url": h.battlenet.AuthURL(state, h.battlenetRedirectURI())})
}

// GET /api/v1/connect/battlenet/callback  (public — browser redirect)
func (h *handlers) connectBattlenetCallback(c *fiber.Ctx) error {
	if h.battlenet == nil || !h.battlenet.Enabled() {
		return c.Redirect("/account?battlenet=error")
	}
	if c.Query("error") != "" {
		return c.Redirect("/account?battlenet=denied")
	}

	userID, ok := verifyState([]byte(h.cfg.JWTSecret), c.Query("state"))
	if !ok {
		return c.Redirect("/account?battlenet=expired")
	}
	code := c.Query("code")
	if code == "" {
		return c.Redirect("/account?battlenet=denied")
	}

	token, err := h.battlenet.ExchangeCode(c.UserContext(), code, h.battlenetRedirectURI())
	if err != nil {
		return c.Redirect("/account?battlenet=denied")
	}
	info, err := h.battlenet.GetUserInfo(c.UserContext(), token.AccessToken)
	if err != nil {
		return c.Redirect("/account?battlenet=denied")
	}

	// One Battle.net account per member.
	taken, err := h.store.ProviderLinkedToOther(c.Context(), "battlenet", info.Sub, userID)
	if err != nil {
		return err
	}
	if taken {
		return c.Redirect("/account?battlenet=conflict")
	}

	account := store.LinkedAccount{Provider: "battlenet", ProviderUserID: info.Sub}
	if info.BattleTag != "" {
		account.DisplayName = strPtr(info.BattleTag)
	}
	// Encrypt the OAuth tokens at rest (AES-256-GCM).
	if h.cipher != nil {
		if enc, err := h.cipher.SealString(token.AccessToken); err == nil {
			account.AccessTokenEnc = enc
		}
		if token.RefreshToken != "" {
			if enc, err := h.cipher.SealString(token.RefreshToken); err == nil {
				account.RefreshTokenEnc = enc
			}
		}
		if token.ExpiresIn > 0 {
			exp := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
			account.TokenExpiresAt = &exp
		}
	}

	if err := h.store.UpsertLinkedAccount(c.Context(), userID, account); err != nil {
		return err
	}
	return c.Redirect("/account?battlenet=linked")
}

// DELETE /api/v1/connect/battlenet  (authenticated)
func (h *handlers) disconnectBattlenet(c *fiber.Ctx) error {
	err := h.store.DeleteLinkedAccount(c.Context(), mustClaims(c).UserID, "battlenet")
	if errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "not_linked", "no Battle.net account linked")
	}
	if err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
