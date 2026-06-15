package api

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/apikey"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

// apiKeyJSON renders a key's metadata. It never includes the secret.
func apiKeyJSON(k store.APIKey) fiber.Map {
	m := fiber.Map{
		"id":         k.ID,
		"label":      k.Label,
		"key_prefix": k.KeyPrefix,
		"can_invite": k.CanInvite,
		"created_at": k.CreatedAt.UTC(),
		"revoked":    k.RevokedAt != nil,
	}
	if k.LastUsedAt != nil {
		m["last_used_at"] = k.LastUsedAt.UTC()
	}
	return m
}

// POST /api/v1/admin/api-keys  (admin) - mint a key; the secret is returned once.
func (h *handlers) createAPIKey(c *fiber.Ctx) error {
	var req struct {
		Label     string `json:"label"`
		CanInvite bool   `json:"can_invite"`
	}
	if err := c.BodyParser(&req); err != nil {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed", "invalid JSON body")
	}
	if req.Label == "" {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed", "label is required")
	}
	full, prefix, hash, err := apikey.Generate()
	if err != nil {
		return err
	}
	id, err := h.store.CreateAPIKey(c.Context(), req.Label, prefix, hash, mustClaims(c).UserID, req.CanInvite)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":         id,
		"label":      req.Label,
		"key_prefix": prefix,
		"key":        full, // shown exactly once
	})
}

// GET /api/v1/admin/api-keys  (admin)
func (h *handlers) listAPIKeys(c *fiber.Ctx) error {
	keys, err := h.store.ListAPIKeys(c.Context())
	if err != nil {
		return err
	}
	out := make([]fiber.Map, len(keys))
	for i, k := range keys {
		out[i] = apiKeyJSON(k)
	}
	return c.JSON(fiber.Map{"keys": out})
}

// DELETE /api/v1/admin/api-keys/:id  (admin) - revoke.
func (h *handlers) revokeAPIKey(c *fiber.Ctx) error {
	err := h.store.RevokeAPIKey(c.Context(), c.Params("id"))
	if errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "not_found", "API key not found")
	}
	if err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
