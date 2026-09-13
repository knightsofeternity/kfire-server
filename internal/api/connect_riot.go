package api

import (
	"errors"
	"log/slog"
	"slices"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/connectors/riot"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

// maxRiotIDBytes bounds the typed Riot ID before it reaches Riot.
const maxRiotIDBytes = 100

// validPlatform reports whether p is a League platform KFIRE routes to. The
// value reaches a URL path, so it is checked against the known list rather than
// escaped.
func validPlatform(p string) bool {
	return slices.Contains(riot.KnownPlatforms(), p)
}

// splitRiotID splits a typed Riot ID, "Name#TAG", on its LAST '#': a Riot game
// name may itself contain '#', so only the last separator is authoritative.
// Each half is trimmed of surrounding whitespace independently, since a member
// pasting a Riot ID often carries a stray space next to the '#'. ok is false
// when there is no '#', either half is empty once trimmed, or the input is
// implausibly long.
func splitRiotID(s string) (gameName, tagLine string, ok bool) {
	if len(s) > maxRiotIDBytes {
		return "", "", false
	}
	i := strings.LastIndex(s, "#")
	if i < 0 {
		return "", "", false
	}
	gameName = strings.TrimSpace(s[:i])
	tagLine = strings.TrimSpace(s[i+1:])
	if gameName == "" || tagLine == "" {
		return "", "", false
	}
	return gameName, tagLine, true
}

// POST /api/v1/connect/riot  (authenticated)
//
// Links the caller's KFIRE account to a Riot account resolved from a typed
// Riot ID. This Riot product carries only an API key, with no RSO
// application, so there is no OAuth flow to prove ownership: linking trusts
// the Riot ID the member types, the same way public stats sites do.
func (h *handlers) connectRiot(c *fiber.Ctx) error {
	if h.riot == nil || !h.riot.Enabled() {
		return errorJSON(c, fiber.StatusNotImplemented, "connector_disabled",
			"the Riot connector is not configured on this instance")
	}

	var body struct {
		RiotID string `json:"riot_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return errorJSON(c, fiber.StatusBadRequest, "invalid_riot_id",
			"riot_id must be \"Name#TAG\"")
	}
	gameName, tagLine, ok := splitRiotID(body.RiotID)
	if !ok {
		return errorJSON(c, fiber.StatusBadRequest, "invalid_riot_id",
			"riot_id must be \"Name#TAG\"")
	}

	userID := mustClaims(c).UserID
	acc, err := h.riot.AccountByRiotID(c.UserContext(), gameName, tagLine)
	if riot.NotFound(err) {
		return errorJSON(c, fiber.StatusNotFound, "riot_id_not_found",
			"Riot does not know this Riot ID")
	}
	if err != nil {
		return err
	}

	// One Riot account per member.
	taken, err := h.store.ProviderLinkedToOther(c.Context(), "riot", acc.PUUID, userID)
	if err != nil {
		return err
	}
	if taken {
		return errorJSON(c, fiber.StatusConflict, "already_linked",
			"this Riot account is already linked to another member")
	}

	// Resolve the League platform BEFORE writing anything. A failure is not
	// fatal: fall back to EUW and let the member correct it from the account
	// page. Resolving first keeps the two rows, identity and routing, from
	// ever being written apart.
	platform := "euw1"
	if p, err := h.riot.ActiveRegion(c.UserContext(), acc.PUUID); err != nil {
		slog.Warn("riot: resolve active region", "user_id", userID, "err", err)
	} else if validPlatform(p) {
		platform = p
	}

	account := store.LinkedAccount{
		Provider:       "riot",
		ProviderUserID: acc.PUUID,
		DisplayName:    strPtr(acc.RiotID()),
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
	return c.JSON(fiber.Map{"riot_id": acc.RiotID(), "platform": platform})
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
