package api

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/connectors/gog"
	"github.com/knightsofeternity/kfire-server/internal/gogsync"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

func (h *handlers) gogEnabled() bool { return h.gog != nil && h.gog.Conn().Enabled() }

func (h *handlers) gogDisabled(c *fiber.Ctx) error {
	return errorJSON(c, fiber.StatusNotImplemented, "connector_disabled",
		"the GOG connector is not configured on this instance")
}

// GET /api/v1/connect/gog  (authenticated)
func (h *handlers) gogStatus(c *fiber.Ctx) error {
	if !h.gogEnabled() {
		return h.gogDisabled(c)
	}
	out := fiber.Map{"linked": false, "login_url": h.gog.Conn().LoginURL()}
	userID := mustClaims(c).UserID
	acc, err := h.store.GetLinkedAccount(c.Context(), userID, "gog")
	if err != nil {
		return c.JSON(out)
	}
	out["linked"] = true
	out["display_name"] = acc.DisplayName
	if st, err := h.store.GogAccountFor(c.Context(), userID); err == nil {
		out["status"] = st.Status
		out["last_synced_at"] = st.LastSyncedAt
	}
	return c.JSON(out)
}

// POST /api/v1/connect/gog {code}  (authenticated, rate limited)
// code is the address of the page GOG ends on after sign-in, or the code alone.
func (h *handlers) connectGog(c *fiber.Ctx) error {
	if !h.gogEnabled() {
		return h.gogDisabled(c)
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := c.BodyParser(&body); err != nil || len(body.Code) > 4096 {
		return errorJSON(c, fiber.StatusBadRequest, "gog_code_invalid", "paste the address GOG ended on after signing in")
	}
	userID := mustClaims(c).UserID
	name, err := h.gog.Link(c.Context(), userID, body.Code)
	switch {
	case errors.Is(err, gogsync.ErrNoCode), errors.Is(err, gog.ErrInvalidGrant):
		return errorJSON(c, fiber.StatusBadRequest, "gog_code_invalid",
			"this code is expired or already used: sign in to GOG again and paste again")
	case errors.Is(err, gogsync.ErrTaken):
		return errorJSON(c, fiber.StatusConflict, "already_linked",
			"this GOG account is already linked to another member")
	case errors.Is(err, gog.ErrClientRejected):
		slog.Error("gog: Galaxy client rejected, check KFIRE_GOG_CLIENT_ID/SECRET")
		return errorJSON(c, fiber.StatusBadGateway, "gog_unavailable", "GOG did not accept the request")
	case err != nil:
		slog.Warn("gog: link", "user_id", userID, "err", err)
		return errorJSON(c, fiber.StatusBadGateway, "gog_unavailable", "GOG did not answer")
	}
	// First import right away, out of the request.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := h.gog.SyncUser(ctx, userID); err != nil {
			slog.Warn("gog: first sync", "user_id", userID, "err", err)
		}
	}()
	return c.JSON(fiber.Map{"display_name": name})
}

// POST /api/v1/connect/gog/sync  (authenticated, rate limited)
func (h *handlers) syncGog(c *fiber.Ctx) error {
	if !h.gogEnabled() {
		return h.gogDisabled(c)
	}
	userID := mustClaims(c).UserID
	err := h.gog.SyncUser(c.Context(), userID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return errorJSON(c, fiber.StatusNotFound, "not_linked", "no GOG account linked")
	case errors.Is(err, gogsync.ErrNeedsRelink):
		return errorJSON(c, fiber.StatusConflict, "gog_needs_relink", "sign in to GOG again to keep syncing")
	case err != nil:
		slog.Warn("gog: manual sync", "user_id", userID, "err", err)
		return errorJSON(c, fiber.StatusBadGateway, "gog_unavailable", "GOG did not answer")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// DELETE /api/v1/connect/gog  (authenticated)
// Imported hours stay, like Steam and Epic.
func (h *handlers) disconnectGog(c *fiber.Ctx) error {
	err := h.store.DeleteGogLink(c.Context(), mustClaims(c).UserID)
	if errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "not_linked", "no GOG account linked")
	}
	if err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
