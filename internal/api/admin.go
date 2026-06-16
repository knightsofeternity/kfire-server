package api

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

// inviteTTL is how long an invite link stays valid.
const inviteTTL = 14 * 24 * time.Hour

// GET /api/v1/config  (public) - lets the SPA tailor the sign-up UI.
func (h *handlers) publicConfig(c *fiber.Ctx) error {
	count, err := h.store.CountUsers(c.Context())
	if err != nil {
		return err
	}
	branding, err := h.store.GetBranding(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{
		"open_registration": h.cfg.OpenRegistration,
		"org_name":          h.cfg.OrgName,
		// True only on a brand-new instance: the first account can be created
		// (it becomes the admin) even when registration is invite-only.
		"needs_setup": count == 0,
		// Branding the SPA applies before/at load.
		"accent":   branding.Accent,
		"has_logo": branding.HasLogo,
		// Which account connectors are configured on this instance, so the SPA
		// only offers the link buttons it can actually fulfil.
		"connectors": fiber.Map{
			"steam":     h.steam != nil && h.steam.Enabled(),
			"battlenet": h.battlenet != nil && h.battlenet.Enabled(),
			"xbox":      h.xbox != nil && h.xbox.Enabled(),
		},
	})
}

// GET /api/v1/admin/members  (admin)
func (h *handlers) listMembers(c *fiber.Ctx) error {
	users, err := h.store.ListUsers(c.Context())
	if err != nil {
		return err
	}
	out := make([]fiber.Map, len(users))
	for i, u := range users {
		m := fiber.Map{
			"id":         u.ID,
			"username":   u.Username,
			"email":      u.Email,
			"role":       u.Role,
			"banned":     u.BannedAt != nil,
			"created_at": u.CreatedAt.UTC(),
		}
		if u.AvatarURL != nil {
			m["avatar_url"] = *u.AvatarURL
		}
		out[i] = m
	}
	return c.JSON(fiber.Map{"members": out})
}

// PATCH /api/v1/admin/members/:id  (admin) - change role and/or ban state.
func (h *handlers) patchMember(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		Role   *string `json:"role"`
		Banned *bool   `json:"banned"`
	}
	if err := c.BodyParser(&req); err != nil {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed", "invalid JSON body")
	}

	target, err := h.store.GetUserByID(c.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "not_found", "user not found")
	}
	if err != nil {
		return err
	}
	self := mustClaims(c).UserID == id

	if req.Role != nil {
		if *req.Role != "admin" && *req.Role != "member" {
			return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed",
				"role must be admin or member")
		}
		// Don't let the instance lose its last admin.
		if target.Role == "admin" && *req.Role != "admin" {
			admins, err := h.store.CountAdmins(c.Context())
			if err != nil {
				return err
			}
			if admins <= 1 {
				return errorJSON(c, fiber.StatusConflict, "last_admin",
					"cannot demote the last admin")
			}
		}
		if err := h.store.SetUserRole(c.Context(), id, *req.Role); err != nil {
			return err
		}
	}

	if req.Banned != nil {
		if self {
			return errorJSON(c, fiber.StatusConflict, "cannot_ban_self", "you cannot ban yourself")
		}
		if *req.Banned && target.Role == "admin" {
			admins, err := h.store.CountAdmins(c.Context())
			if err != nil {
				return err
			}
			if admins <= 1 {
				return errorJSON(c, fiber.StatusConflict, "last_admin",
					"cannot ban the last admin")
			}
		}
		if err := h.store.SetUserBanned(c.Context(), id, *req.Banned); err != nil {
			return err
		}
	}

	updated, err := h.store.GetUserByID(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{
		"id":       updated.ID,
		"username": updated.Username,
		"role":     updated.Role,
		"banned":   updated.BannedAt != nil,
	})
}

// GET /api/v1/admin/invites  (admin)
func (h *handlers) listInvites(c *fiber.Ctx) error {
	invites, err := h.store.ListPendingInvites(c.Context())
	if err != nil {
		return err
	}
	out := make([]fiber.Map, len(invites))
	for i, inv := range invites {
		out[i] = inviteJSON(h.cfg.PublicURL, inv)
	}
	return c.JSON(fiber.Map{"invites": out})
}

// POST /api/v1/admin/invites  (admin) - create a shareable invite link.
func (h *handlers) createInvite(c *fiber.Ctx) error {
	var req struct {
		Note string `json:"note"`
		Role string `json:"role"`
	}
	_ = c.BodyParser(&req)
	if req.Role == "" {
		req.Role = "member"
	}
	if req.Role != "admin" && req.Role != "member" {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed",
			"role must be admin or member")
	}

	code, err := newInviteCode()
	if err != nil {
		return err
	}
	uid := mustClaims(c).UserID
	if err := h.store.CreateInvite(c.Context(), code, req.Note, req.Role,
		&uid, time.Now().Add(inviteTTL)); err != nil {
		return err
	}
	inv := store.Invite{
		Code:      code,
		Role:      req.Role,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().Add(inviteTTL).UTC(),
	}
	if req.Note != "" {
		inv.Note = &req.Note
	}
	return c.Status(fiber.StatusCreated).JSON(inviteJSON(h.cfg.PublicURL, inv))
}

// DELETE /api/v1/admin/invites/:code  (admin)
func (h *handlers) deleteInvite(c *fiber.Ctx) error {
	err := h.store.DeleteInvite(c.Context(), c.Params("code"))
	if errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "not_found", "invite not found")
	}
	if err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func inviteJSON(publicURL string, inv store.Invite) fiber.Map {
	m := fiber.Map{
		"code":       inv.Code,
		"role":       inv.Role,
		"url":        inviteURL(publicURL, inv.Code),
		"created_at": inv.CreatedAt.UTC(),
		"expires_at": inv.ExpiresAt.UTC(),
	}
	if inv.Note != nil {
		m["note"] = *inv.Note
	}
	return m
}

// newInviteCode returns a URL-safe 128-bit random code.
func newInviteCode() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
