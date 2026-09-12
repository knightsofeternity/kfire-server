package api

import (
	"errors"
	"log/slog"
	"slices"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/connectors/riot"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

func (h *handlers) riotRedirectURI() string {
	return h.cfg.PublicURL + "/api/v1/connect/riot/callback"
}

// validPlatform reports whether p is a League platform KFIRE routes to. The
// value reaches a URL path, so it is checked against the known list rather than
// escaped.
func validPlatform(p string) bool {
	return slices.Contains(riot.KnownPlatforms(), p)
}

// GET /api/v1/connect/riot  (authenticated)
//
// Returns the RSO authorization URL for the SPA to navigate to.
func (h *handlers) connectRiotStart(c *fiber.Ctx) error {
	if h.riot == nil || !h.riot.Enabled() {
		return errorJSON(c, fiber.StatusNotImplemented, "connector_disabled",
			"the Riot connector is not configured on this instance")
	}
	state := signState([]byte(h.cfg.JWTSecret), mustClaims(c).UserID)
	return c.JSON(fiber.Map{"url": h.riot.AuthURL(state, h.riotRedirectURI())})
}

// GET /api/v1/connect/riot/callback  (public - browser redirect)
//
// Exchanges the code, reads the PUUID, then discards the tokens: every later
// read uses the server's API key.
func (h *handlers) connectRiotCallback(c *fiber.Ctx) error {
	if h.riot == nil || !h.riot.Enabled() {
		return c.Redirect("/account?riot=error")
	}
	if c.Query("error") != "" {
		return c.Redirect("/account?riot=denied")
	}
	userID, ok := verifyState([]byte(h.cfg.JWTSecret), c.Query("state"))
	if !ok {
		return c.Redirect("/account?riot=expired")
	}
	code := c.Query("code")
	if code == "" {
		return c.Redirect("/account?riot=denied")
	}

	token, err := h.riot.ExchangeCode(c.UserContext(), code, h.riotRedirectURI())
	if err != nil {
		slog.Warn("riot: exchange code", "err", err)
		return c.Redirect("/account?riot=denied")
	}
	puuid, err := h.riot.UserPUUID(c.UserContext(), token)
	if err != nil {
		slog.Warn("riot: read puuid", "err", err)
		return c.Redirect("/account?riot=denied")
	}

	// One Riot account per member.
	taken, err := h.store.ProviderLinkedToOther(c.Context(), "riot", puuid, userID)
	if err != nil {
		return err
	}
	if taken {
		return c.Redirect("/account?riot=conflict")
	}

	// Resolve the League platform BEFORE writing anything. A failure is not
	// fatal: fall back to EUW and let the member correct it from the account
	// page. Resolving first keeps the two rows, identity and routing, from
	// ever being written apart.
	platform := "euw1"
	if p, err := h.riot.ActiveRegion(c.UserContext(), puuid); err != nil {
		slog.Warn("riot: resolve active region", "user_id", userID, "err", err)
	} else if validPlatform(p) {
		platform = p
	}

	account := store.LinkedAccount{Provider: "riot", ProviderUserID: puuid}
	if acc, err := h.riot.AccountByPUUID(c.UserContext(), puuid); err == nil {
		account.DisplayName = strPtr(acc.RiotID())
	}
	if err := h.store.UpsertLinkedAccount(c.Context(), userID, account); err != nil {
		return err
	}
	if err := h.store.UpsertRiotRouting(c.Context(), userID, platform,
		riot.MatchCluster(platform), "auto"); err != nil {
		// RiotAccountFor joins identity and routing, so an identity row without
		// a routing row reads as "not linked" and leaves the member stuck with
		// a link they cannot use or see. Roll the identity back instead.
		if derr := h.store.DeleteLinkedAccount(c.Context(), userID, "riot"); derr != nil {
			slog.Error("riot: roll back orphaned link", "user_id", userID, "err", derr)
		}
		return err
	}
	return c.Redirect("/account?riot=linked")
}

// DELETE /api/v1/connect/riot  (authenticated)
func (h *handlers) disconnectRiot(c *fiber.Ctx) error {
	userID := mustClaims(c).UserID
	// Routing and profiles go first. Deleting them is a no-op when there is
	// nothing to delete, so a failure here leaves the identity row in place and
	// the member can simply retry. The reverse order would strand those rows:
	// with the identity already gone, a second attempt answers "not linked" and
	// never reaches them again.
	if err := h.store.DeleteRiotData(c.Context(), userID); err != nil {
		return err
	}
	err := h.store.DeleteLinkedAccount(c.Context(), userID, "riot")
	if errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "not_linked", "no Riot account linked")
	}
	if err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// GET /api/v1/connect/riot/region  (authenticated)
//
// Returns the member's current League platform, so the account page's picker
// opens on the detected region rather than on a default.
//
// The platform deliberately does not ride along in connectionJSON: that shape
// is shared with the public API, and routing is internal plumbing no third
// party needs.
func (h *handlers) riotRegion(c *fiber.Ctx) error {
	acc, err := h.store.RiotAccountFor(c.Context(), mustClaims(c).UserID)
	if err != nil {
		return errorJSON(c, fiber.StatusNotFound, "not_linked", "no Riot account linked")
	}
	return c.JSON(fiber.Map{"platform": acc.Platform, "source": acc.RegionSource})
}

// PATCH /api/v1/connect/riot/region  (authenticated)
//
// Corrects the member's League platform when the automatic resolution was wrong
// or unavailable. Marks the routing manual so it is never overwritten.
func (h *handlers) updateRiotRegion(c *fiber.Ctx) error {
	var body struct {
		Platform string `json:"platform"`
	}
	if err := c.BodyParser(&body); err != nil || !validPlatform(body.Platform) {
		return errorJSON(c, fiber.StatusBadRequest, "invalid_platform",
			"platform must be one of the supported League regions")
	}
	userID := mustClaims(c).UserID
	if _, err := h.store.RiotAccountFor(c.Context(), userID); err != nil {
		return errorJSON(c, fiber.StatusNotFound, "not_linked", "no Riot account linked")
	}
	if err := h.store.UpsertRiotRouting(c.Context(), userID, body.Platform,
		riot.MatchCluster(body.Platform), "manual"); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"platform": body.Platform})
}
