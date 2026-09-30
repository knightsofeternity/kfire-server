package api

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/connectors/psn"
)

// GET /api/v1/admin/psn
func (h *handlers) adminPsnStatus(c *fiber.Ctx) error {
	if h.psn == nil {
		return errorJSON(c, fiber.StatusNotImplemented, "connector_disabled",
			"the PlayStation connector is not built into this server")
	}
	st, err := h.psn.Bot().Status(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(st)
}

// PUT /api/v1/admin/psn {npsso}
//
// The NPSSO is proven against Sony before it is stored: a wrong or revoked one
// is refused here, in front of the admin. It is never sent back.
func (h *handlers) adminSetPsnNPSSO(c *fiber.Ctx) error {
	var body struct {
		NPSSO string `json:"npsso"`
	}
	if err := c.BodyParser(&body); err != nil {
		return errorJSON(c, fiber.StatusBadRequest, "invalid_npsso", "paste the npsso value")
	}
	npsso := strings.TrimSpace(body.NPSSO)
	// The admin may paste the whole {"npsso":"…"} page Sony shows.
	if i := strings.Index(npsso, `"npsso":"`); i >= 0 {
		npsso = strings.SplitN(npsso[i+9:], `"`, 2)[0]
	}
	if len(npsso) != 64 {
		return errorJSON(c, fiber.StatusBadRequest, "invalid_npsso", "an NPSSO is 64 characters long")
	}
	if h.psn == nil {
		return errorJSON(c, fiber.StatusNotImplemented, "connector_disabled",
			"the PlayStation connector is not built into this server")
	}
	err := h.psn.Bot().SetNPSSO(c.Context(), npsso)
	if errors.Is(err, psn.ErrLoginRequired) {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "npsso_refused",
			"Sony refused this NPSSO: log in again on playstation.com and copy a fresh one")
	}
	if err != nil {
		return errorJSON(c, fiber.StatusBadGateway, "psn_unavailable", "PlayStation did not answer")
	}
	return h.adminPsnStatus(c)
}
