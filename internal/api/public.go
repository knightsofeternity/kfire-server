// Package api — public.go wires /api/public/v1, the read-only API-key surface.
// Handlers evaluate privacy as a non-privileged viewer (viewer id "", role
// "member"), so members' activity_visible / sessions_visible toggles apply.
package api

import (
	"github.com/gofiber/fiber/v2"
)

// publicViewerRole is the role used for all key-authenticated reads: never
// self, never admin, so privacy toggles are honoured.
const publicViewerRole = "member"

// GET /api/public/v1/presence
func (h *handlers) publicPresence(c *fiber.Ctx) error {
	rows, err := h.store.ListPresence(c.Context())
	if err != nil {
		return err
	}
	entries := make([]fiber.Map, 0, len(rows))
	for _, r := range rows {
		online := h.hub.OnlineSince(r.UserID)
		showGame := r.ActivityVisible // public viewer: only when the member opted in
		entries = append(entries, h.presenceEntry(r.UserID, r.Username, r.AvatarURL, r.Game, r.StartedAt, online, showGame))
	}
	return c.JSON(fiber.Map{"entries": entries})
}
