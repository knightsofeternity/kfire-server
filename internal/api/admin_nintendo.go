package api

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/connectors/nintendo"
	"github.com/knightsofeternity/kfire-server/internal/nintendosync"
)

func (h *handlers) nintendoBuilt(c *fiber.Ctx) bool {
	if h.nintendo == nil || !h.nintendo.Bot().Enabled() {
		_ = errorJSON(c, fiber.StatusNotImplemented, "connector_disabled",
			"the Nintendo sidecar is not configured on this instance (KFIRE_NXAPI_URL)")
		return false
	}
	return true
}

// GET /api/v1/admin/nintendo
func (h *handlers) adminNintendoStatus(c *fiber.Ctx) error {
	if !h.nintendoBuilt(c) {
		return nil
	}
	st, err := h.nintendo.Bot().Status(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(st)
}

// POST /api/v1/admin/nintendo/login → {url}
func (h *handlers) adminNintendoStartLogin(c *fiber.Ctx) error {
	if !h.nintendoBuilt(c) {
		return nil
	}
	url, err := h.nintendo.Bot().StartLogin(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"url": url})
}

// PUT /api/v1/admin/nintendo {link}
//
// The npf link copied from Nintendo's "Select this account" button. The
// session it gives is proven through the sidecar before anything is stored.
func (h *handlers) adminNintendoFinishLogin(c *fiber.Ctx) error {
	var body struct {
		Link string `json:"link"`
	}
	if err := c.BodyParser(&body); err != nil || body.Link == "" {
		return errorJSON(c, fiber.StatusBadRequest, "invalid_link", "paste the npf71b963c1b7b6d119://auth link")
	}
	if _, _, err := nintendo.ParseRedirect(body.Link); err != nil {
		return errorJSON(c, fiber.StatusBadRequest, "invalid_link", "paste the npf71b963c1b7b6d119://auth link")
	}
	if !h.nintendoBuilt(c) {
		return nil
	}
	err := h.nintendo.Bot().FinishLogin(c.Context(), body.Link)
	if errors.Is(err, nintendosync.ErrLoginExpired) {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "login_expired",
			"this link is for another or an expired login: generate a new one")
	}
	if err != nil {
		return errorJSON(c, fiber.StatusBadGateway, "nintendo_unavailable",
			"Nintendo refused this link or did not answer: generate a new one")
	}
	return h.adminNintendoStatus(c)
}
