package api

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/connectors/psn"
	"github.com/knightsofeternity/kfire-server/internal/psnsync"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

const maxPsnIDBytes = 32 // online ids are 3 to 16 characters

// psnReady returns the bot's token, or ok=false once it has written the
// error response itself (callers then return nil).
func (h *handlers) psnReady(c *fiber.Ctx) (tok string, ok bool) {
	if h.psn == nil || !h.psn.Bot().Configured(c.Context()) {
		_ = errorJSON(c, fiber.StatusNotImplemented, "connector_disabled",
			"the PlayStation connector is not configured on this instance")
		return "", false
	}
	tok, err := h.psn.Bot().Token(c.Context())
	if errors.Is(err, psnsync.ErrNeedsNPSSO) {
		_ = errorJSON(c, fiber.StatusServiceUnavailable, "psn_bot_down",
			"the PlayStation bot must be reconnected by an admin")
		return "", false
	}
	if err != nil {
		slog.Warn("psn: token", "err", err)
		_ = errorJSON(c, fiber.StatusBadGateway, "psn_unavailable", "PlayStation did not answer")
		return "", false
	}
	return tok, true
}

// GET /api/v1/connect/psn  (authenticated)
func (h *handlers) psnStatus(c *fiber.Ctx) error {
	if h.psn == nil || !h.psn.Bot().Configured(c.Context()) {
		return errorJSON(c, fiber.StatusNotImplemented, "connector_disabled",
			"the PlayStation connector is not configured on this instance")
	}
	out := fiber.Map{"linked": false, "bot_online_id": h.psn.Bot().OnlineID(c.Context())}
	acc, err := h.store.GetLinkedAccount(c.Context(), mustClaims(c).UserID, "psn")
	if err != nil {
		return c.JSON(out)
	}
	out["linked"] = true
	out["online_id"] = acc.DisplayName
	out["friend"] = false
	if tok, err := h.psn.Bot().Token(c.Context()); err == nil {
		if friends, err := h.psn.Bot().Conn().Friends(c.Context(), tok); err == nil {
			for _, f := range friends {
				if f == acc.ProviderUserID {
					out["friend"] = true
				}
			}
		}
	}
	return c.JSON(out)
}

// POST /api/v1/connect/psn {online_id}  (authenticated)
func (h *handlers) connectPsn(c *fiber.Ctx) error {
	var body struct {
		OnlineID string `json:"online_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return errorJSON(c, fiber.StatusBadRequest, "invalid_psn_id", "online_id must be your PSN online ID")
	}
	id := strings.TrimSpace(body.OnlineID)
	if len(id) < 3 || len(id) > maxPsnIDBytes {
		return errorJSON(c, fiber.StatusBadRequest, "invalid_psn_id", "online_id must be your PSN online ID")
	}
	tok, ok := h.psnReady(c)
	if !ok {
		return nil
	}
	userID := mustClaims(c).UserID
	p, err := h.psn.Bot().Conn().Lookup(c.Context(), tok, id)
	if errors.Is(err, psn.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "psn_not_found", "PlayStation does not know this online ID")
	}
	if err != nil {
		slog.Warn("psn: lookup", "user_id", userID, "err", err)
		return errorJSON(c, fiber.StatusBadGateway, "psn_unavailable", "PlayStation did not answer")
	}
	taken, err := h.store.ProviderLinkedToOther(c.Context(), "psn", p.AccountID, userID)
	if err != nil {
		return err
	}
	if taken {
		return errorJSON(c, fiber.StatusConflict, "already_linked",
			"this PlayStation account is already linked to another member")
	}
	if err := h.store.UpsertLinkedAccount(c.Context(), userID, store.LinkedAccount{
		Provider: "psn", ProviderUserID: p.AccountID, DisplayName: strPtr(p.OnlineID),
	}); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"online_id": p.OnlineID, "bot_online_id": h.psn.Bot().OnlineID(c.Context())})
}

// DELETE /api/v1/connect/psn  (authenticated)
//
// Drops the link and closes a console session in progress. The bot also ends
// the friendship, best effort. Imported hours and trophies stay, like Steam.
func (h *handlers) disconnectPsn(c *fiber.Ctx) error {
	userID := mustClaims(c).UserID
	acc, err := h.store.GetLinkedAccount(c.Context(), userID, "psn")
	if errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "not_linked", "no PlayStation account linked")
	}
	if err != nil {
		return err
	}
	if err := h.store.DeleteLinkedAccount(c.Context(), userID, "psn"); err != nil {
		return err
	}
	if open, err := h.store.OpenSessionBySource(c.Context(), userID, "psn_api"); err == nil && open != nil {
		_, _ = h.store.EndSession(c.Context(), userID, open.Game.ID)
	}
	if h.psn != nil {
		if tok, err := h.psn.Bot().Token(c.Context()); err == nil {
			if err := h.psn.Bot().Conn().RemoveFriend(c.Context(), tok, acc.ProviderUserID); err != nil {
				slog.Warn("psn: remove friend on unlink", "user_id", userID, "err", err)
			}
		}
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// POST /api/v1/connect/psn/sync  (authenticated, rate limited)
func (h *handlers) syncPsn(c *fiber.Ctx) error {
	if _, ok := h.psnReady(c); !ok {
		return nil
	}
	userID := mustClaims(c).UserID
	acc, err := h.store.GetLinkedAccount(c.Context(), userID, "psn")
	if err != nil {
		return errorJSON(c, fiber.StatusNotFound, "not_linked", "no PlayStation account linked")
	}
	if err := h.psn.SyncLibrary(c.Context(), userID, acc.ProviderUserID); err != nil {
		if errors.Is(err, psn.ErrForbidden) {
			return errorJSON(c, fiber.StatusConflict, "psn_not_friend",
				"add the bot as a friend on PlayStation first")
		}
		slog.Warn("psn: manual sync", "user_id", userID, "err", err)
		return errorJSON(c, fiber.StatusBadGateway, "psn_unavailable", "PlayStation did not answer")
	}
	return c.SendStatus(fiber.StatusNoContent)
}
