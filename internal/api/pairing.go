package api

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

// pairingTTL bounds how long a device-linking request stays valid.
const pairingTTL = 10 * time.Minute

// userCodeAlphabet excludes visually ambiguous characters (0/O, 1/I, etc.).
const userCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// POST /api/v1/devices/pair/start  (public)
//
// A client begins linking. Returns a device_code (secret, kept by the client)
// and a user_code shown to the user, plus the verification URL to open.
func (h *handlers) pairStart(c *fiber.Ctx) error {
	var req struct {
		DeviceID string `json:"device_id"`
		Name     string `json:"name"`
		Platform string `json:"platform"`
	}
	if err := c.BodyParser(&req); err != nil {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed", "invalid JSON body")
	}
	if _, err := uuid.Parse(req.DeviceID); err != nil {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed", "device_id must be a UUID")
	}
	switch req.Platform {
	case "windows", "macos", "linux":
	default:
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed",
			"platform must be windows, macos or linux")
	}
	if req.Name == "" || len(req.Name) > 64 {
		req.Name = "Desktop"
	}

	deviceCode, err := randToken(32)
	if err != nil {
		return err
	}
	userCode, err := randUserCode(8)
	if err != nil {
		return err
	}

	if err := h.store.CreatePairing(c.Context(), store.Pairing{
		DeviceCode: deviceCode,
		UserCode:   userCode,
		DeviceID:   req.DeviceID,
		DeviceName: req.Name,
		Platform:   req.Platform,
		ExpiresAt:  time.Now().Add(pairingTTL),
	}); err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"device_code":      deviceCode,
		"user_code":        userCode,
		"verification_url": h.cfg.PublicURL + "/link?code=" + userCode,
		"expires_in":       int(pairingTTL.Seconds()),
		"interval":         3,
	})
}

// GET /api/v1/devices/pair/:code  (authenticated)
//
// The approval page reads this to show what's being linked.
func (h *handlers) pairInfo(c *fiber.Ctx) error {
	p, err := h.store.GetPendingPairingByUserCode(c.Context(), c.Params("code"))
	if errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "invalid_code", "this link code is invalid or expired")
	}
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"device_name": p.DeviceName, "platform": p.Platform})
}

// POST /api/v1/devices/pair/:code/approve  (authenticated)
func (h *handlers) pairApprove(c *fiber.Ctx) error {
	err := h.store.ApprovePairing(c.Context(), c.Params("code"), mustClaims(c).UserID)
	if errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "invalid_code", "this link code is invalid or expired")
	}
	if err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// POST /api/v1/devices/pair/poll  (public)
//
// The client polls with its device_code until the pairing is approved, then
// receives device-bound tokens (once).
func (h *handlers) pairPoll(c *fiber.Ctx) error {
	var req struct {
		DeviceCode string `json:"device_code"`
	}
	if err := c.BodyParser(&req); err != nil || req.DeviceCode == "" {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed", "missing device_code")
	}

	p, err := h.store.GetPairingByDeviceCode(c.Context(), req.DeviceCode)
	if errors.Is(err, store.ErrNotFound) {
		return c.JSON(fiber.Map{"status": "denied"})
	}
	if err != nil {
		return err
	}
	if time.Now().After(p.ExpiresAt) {
		return c.JSON(fiber.Map{"status": "expired"})
	}

	switch p.Status {
	case "pending":
		return c.JSON(fiber.Map{"status": "pending"})
	case "claimed":
		return c.JSON(fiber.Map{"status": "denied"}) // already used
	case "approved":
		u, err := h.store.GetUserByID(c.Context(), *p.UserID)
		if err != nil || u.BannedAt != nil {
			return c.JSON(fiber.Map{"status": "denied"})
		}
		tokens, err := h.mintTokens(c, u, deviceInfo{
			DeviceID: p.DeviceID, Name: p.DeviceName, Platform: p.Platform,
		})
		if err != nil {
			return err
		}
		// Single use: claim it now that the tokens are minted.
		_ = h.store.ClaimPairing(c.Context(), req.DeviceCode)
		tokens["status"] = "complete"
		return c.JSON(tokens)
	default:
		return c.JSON(fiber.Map{"status": "pending"})
	}
}

func randToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// randUserCode returns a short human-friendly code formatted as XXXX-XXXX.
func randUserCode(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, 0, n+1)
	for i, b := range buf {
		if i == n/2 {
			out = append(out, '-')
		}
		out = append(out, userCodeAlphabet[int(b)%len(userCodeAlphabet)])
	}
	return string(out), nil
}
