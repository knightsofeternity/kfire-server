package api

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/connectors/epic"
	"github.com/knightsofeternity/kfire-server/internal/epicsync"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

func (h *handlers) epicEnabled() bool { return h.epic != nil && h.epic.Conn().Enabled() }

func (h *handlers) epicDisabled(c *fiber.Ctx) error {
	return errorJSON(c, fiber.StatusNotImplemented, "connector_disabled",
		"the Epic Games connector is not configured on this instance")
}

// GET /api/v1/connect/epic  (authenticated)
func (h *handlers) epicStatus(c *fiber.Ctx) error {
	if !h.epicEnabled() {
		return h.epicDisabled(c)
	}
	out := fiber.Map{"linked": false, "login_url": h.epic.Conn().LoginURL()}
	userID := mustClaims(c).UserID
	acc, err := h.store.GetLinkedAccount(c.Context(), userID, "epic")
	if err != nil {
		return c.JSON(out)
	}
	out["linked"] = true
	out["display_name"] = acc.DisplayName
	if st, err := h.store.EpicAccountFor(c.Context(), userID); err == nil {
		out["status"] = st.Status
		out["last_synced_at"] = st.LastSyncedAt
	}
	return c.JSON(out)
}

// POST /api/v1/connect/epic {code}  (authenticated, rate limited)
// code is what Epic's page shows after sign-in: the JSON or the code alone.
func (h *handlers) connectEpic(c *fiber.Ctx) error {
	if !h.epicEnabled() {
		return h.epicDisabled(c)
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := c.BodyParser(&body); err != nil || len(body.Code) > 4096 {
		return errorJSON(c, fiber.StatusBadRequest, "epic_code_invalid", "paste what Epic showed after signing in")
	}
	userID := mustClaims(c).UserID
	name, err := h.epic.Link(c.Context(), userID, body.Code)
	switch {
	case errors.Is(err, epicsync.ErrNoCode), errors.Is(err, epic.ErrInvalidGrant):
		return errorJSON(c, fiber.StatusBadRequest, "epic_code_invalid",
			"this code is expired or already used: reload the Epic page and paste again")
	case errors.Is(err, epicsync.ErrTaken):
		return errorJSON(c, fiber.StatusConflict, "already_linked",
			"this Epic account is already linked to another member")
	case errors.Is(err, epic.ErrClientRejected):
		slog.Error("epic: launcher client rejected, check KFIRE_EPIC_CLIENT_ID/SECRET")
		return errorJSON(c, fiber.StatusBadGateway, "epic_unavailable", "Epic did not accept the request")
	case err != nil:
		slog.Warn("epic: link", "user_id", userID, "err", err)
		return errorJSON(c, fiber.StatusBadGateway, "epic_unavailable", "Epic did not answer")
	}
	// First import right away, out of the request.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := h.epic.SyncUser(ctx, userID); err != nil {
			slog.Warn("epic: first sync", "user_id", userID, "err", err)
		}
	}()
	return c.JSON(fiber.Map{"display_name": name})
}

// POST /api/v1/connect/epic/sync  (authenticated, rate limited)
func (h *handlers) syncEpic(c *fiber.Ctx) error {
	if !h.epicEnabled() {
		return h.epicDisabled(c)
	}
	userID := mustClaims(c).UserID
	err := h.epic.SyncUser(c.Context(), userID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return errorJSON(c, fiber.StatusNotFound, "not_linked", "no Epic Games account linked")
	case errors.Is(err, epicsync.ErrNeedsRelink):
		return errorJSON(c, fiber.StatusConflict, "epic_needs_relink", "sign in to Epic again to keep syncing")
	case err != nil:
		slog.Warn("epic: manual sync", "user_id", userID, "err", err)
		return errorJSON(c, fiber.StatusBadGateway, "epic_unavailable", "Epic did not answer")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// DELETE /api/v1/connect/epic  (authenticated)
// Imported hours stay, like Steam and PSN.
func (h *handlers) disconnectEpic(c *fiber.Ctx) error {
	err := h.store.DeleteEpicLink(c.Context(), mustClaims(c).UserID)
	if errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "not_linked", "no Epic Games account linked")
	}
	if err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
