package api

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/connectors/nintendo"
	"github.com/knightsofeternity/kfire-server/internal/nintendosync"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

// nintendoReady returns the bot's session, or ok=false once it has written
// the error response itself (callers then return nil).
func (h *handlers) nintendoReady(c *fiber.Ctx) (session string, ok bool) {
	if h.nintendo == nil || !h.nintendo.Bot().Configured(c.Context()) {
		_ = errorJSON(c, fiber.StatusNotImplemented, "connector_disabled",
			"the Nintendo connector is not configured on this instance")
		return "", false
	}
	session, err := h.nintendo.Bot().Session(c.Context())
	if errors.Is(err, nintendosync.ErrNeedsLogin) {
		_ = errorJSON(c, fiber.StatusServiceUnavailable, "nintendo_bot_down",
			"the Nintendo bot must be logged in again by an admin")
		return "", false
	}
	if err != nil {
		slog.Warn("nintendo: session", "err", err)
		_ = errorJSON(c, fiber.StatusBadGateway, "nintendo_unavailable", "Nintendo did not answer")
		return "", false
	}
	return session, true
}

// nintendoFailure answers a failed call to Nintendo.
func (h *handlers) nintendoFailure(c *fiber.Ctx, userID string, err error) error {
	switch {
	case errors.Is(err, nintendo.ErrRateLimited):
		return errorJSON(c, fiber.StatusTooManyRequests, "nintendo_busy",
			"Nintendo asks to slow down, try again in a few minutes")
	case errors.Is(err, nintendo.ErrSessionRevoked):
		h.nintendo.Bot().Revoked(c.Context())
		return errorJSON(c, fiber.StatusServiceUnavailable, "nintendo_bot_down",
			"the Nintendo bot must be logged in again by an admin")
	}
	slog.Warn("nintendo: call", "user_id", userID, "err", err)
	return errorJSON(c, fiber.StatusBadGateway, "nintendo_unavailable", "Nintendo did not answer")
}

// GET /api/v1/connect/nintendo  (authenticated)
func (h *handlers) nintendoStatus(c *fiber.Ctx) error {
	if h.nintendo == nil || !h.nintendo.Bot().Configured(c.Context()) {
		return errorJSON(c, fiber.StatusNotImplemented, "connector_disabled",
			"the Nintendo connector is not configured on this instance")
	}
	name, code := h.nintendo.Bot().Identity(c.Context())
	out := fiber.Map{"linked": false, "bot": fiber.Map{"nickname": name, "friend_code": code}}
	userID := mustClaims(c).UserID
	acc, err := h.store.GetLinkedAccount(c.Context(), userID, "nintendo")
	if err != nil {
		return c.JSON(out)
	}
	out["linked"] = true
	out["nickname"] = acc.DisplayName
	if fc, err := h.store.NintendoFriendCode(c.Context(), userID); err == nil && fc != "" {
		out["friend_code"] = fc
	}
	out["friend"] = false
	if session, err := h.nintendo.Bot().Session(c.Context()); err == nil {
		if friends, err := h.nintendo.Bot().Client().Friends(c.Context(), session); err == nil {
			for _, f := range friends {
				if f.NsaID == acc.ProviderUserID {
					out["friend"] = true
				}
			}
		}
	}
	return c.JSON(out)
}

// POST /api/v1/connect/nintendo {friend_code}  (authenticated)
func (h *handlers) connectNintendo(c *fiber.Ctx) error {
	var body struct {
		FriendCode string `json:"friend_code"`
	}
	if err := c.BodyParser(&body); err != nil {
		return errorJSON(c, fiber.StatusBadRequest, "invalid_friend_code", "friend_code must be SW-xxxx-xxxx-xxxx")
	}
	code, err := nintendo.NormalizeFriendCode(body.FriendCode)
	if err != nil {
		return errorJSON(c, fiber.StatusBadRequest, "invalid_friend_code", "friend_code must be SW-xxxx-xxxx-xxxx")
	}
	session, ok := h.nintendoReady(c)
	if !ok {
		return nil
	}
	userID := mustClaims(c).UserID
	u, err := h.nintendo.Bot().Client().LookupFriendCode(c.Context(), session, code)
	if errors.Is(err, nintendo.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "nintendo_not_found", "Nintendo does not know this friend code")
	}
	if err != nil {
		return h.nintendoFailure(c, userID, err)
	}
	taken, err := h.store.ProviderLinkedToOther(c.Context(), "nintendo", u.NsaID, userID)
	if err != nil {
		return err
	}
	if taken {
		return errorJSON(c, fiber.StatusConflict, "already_linked",
			"this Nintendo account is already linked to another member")
	}
	if err := h.store.UpsertLinkedAccount(c.Context(), userID, store.LinkedAccount{
		Provider: "nintendo", ProviderUserID: u.NsaID, DisplayName: strPtr(u.Name),
	}); err != nil {
		return err
	}
	if err := h.store.SetNintendoFriendCode(c.Context(), userID, code); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"nickname": u.Name, "friend_code": code})
}

// DELETE /api/v1/connect/nintendo  (authenticated)
//
// Drops the link and closes a console session in progress; the bot ends the
// friendship, best effort. Imported hours stay, like Steam.
func (h *handlers) disconnectNintendo(c *fiber.Ctx) error {
	userID := mustClaims(c).UserID
	acc, err := h.store.GetLinkedAccount(c.Context(), userID, "nintendo")
	if errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "not_linked", "no Nintendo account linked")
	}
	if err != nil {
		return err
	}
	if err := h.store.DeleteLinkedAccount(c.Context(), userID, "nintendo"); err != nil {
		return err
	}
	_ = h.store.DeleteNintendoFriendCode(c.Context(), userID)
	if open, err := h.store.OpenSessionBySource(c.Context(), userID, "nintendo_api"); err == nil && open != nil {
		_, _ = h.store.EndSession(c.Context(), userID, open.Game.ID)
	}
	if h.nintendo != nil {
		if session, err := h.nintendo.Bot().Session(c.Context()); err == nil {
			if err := h.nintendo.Bot().Client().DeleteFriend(c.Context(), session, acc.ProviderUserID); err != nil {
				slog.Warn("nintendo: remove friend on unlink", "user_id", userID, "err", err)
			}
		}
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// POST /api/v1/connect/nintendo/sync  (authenticated, rate limited)
func (h *handlers) syncNintendo(c *fiber.Ctx) error {
	if _, ok := h.nintendoReady(c); !ok {
		return nil
	}
	userID := mustClaims(c).UserID
	acc, err := h.store.GetLinkedAccount(c.Context(), userID, "nintendo")
	if err != nil {
		return errorJSON(c, fiber.StatusNotFound, "not_linked", "no Nintendo account linked")
	}
	if err := h.nintendo.SyncLibrary(c.Context(), userID, acc.ProviderUserID); err != nil {
		if errors.Is(err, nintendo.ErrNotFound) {
			return errorJSON(c, fiber.StatusConflict, "nintendo_not_friend",
				"add the bot as a friend on your Switch first")
		}
		return h.nintendoFailure(c, userID, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
