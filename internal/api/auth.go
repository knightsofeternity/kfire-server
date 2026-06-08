package api

import (
	"errors"
	"log/slog"
	"regexp"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/knightsofeternity/kfire-server/internal/auth"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

var (
	usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,32}$`)
	emailRe    = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

// dummyHash is verified against when the user does not exist, so login takes
// the same time whether the username is known or not (timing oracle).
const dummyHash = "$argon2id$v=19$m=65536,t=3,p=2$AAAAAAAAAAAAAAAAAAAAAA$t1AbZWBohJjEZSnvJsOjJfRvUmW2dXfhBQS5sTgVDxA"

type deviceInfo struct {
	DeviceID string `json:"device_id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
}

func (d deviceInfo) validate() string {
	if _, err := uuid.Parse(d.DeviceID); err != nil {
		return "device.device_id must be a UUID"
	}
	if d.Name == "" || len(d.Name) > 64 {
		return "device.name must be 1-64 characters"
	}
	switch d.Platform {
	case "windows", "macos", "linux", "web":
		return ""
	default:
		return "device.platform must be one of windows, macos, linux, web"
	}
}

func userJSON(u store.User, includeEmail bool) fiber.Map {
	m := fiber.Map{
		"id":         u.ID,
		"username":   u.Username,
		"role":       u.Role,
		"created_at": u.CreatedAt.UTC(),
	}
	if u.AvatarURL != nil {
		m["avatar_url"] = *u.AvatarURL
	}
	if includeEmail {
		// Private fields, only returned on the owner's own profile.
		m["email"] = u.Email
		m["activity_visible"] = u.ActivityVisible
	}
	return m
}

// POST /api/v1/auth/register
func (h *handlers) register(c *fiber.Ctx) error {
	var req struct {
		Username   string `json:"username"`
		Email      string `json:"email"`
		Password   string `json:"password"`
		InviteCode string `json:"invite_code"`
	}
	if err := c.BodyParser(&req); err != nil {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed", "invalid JSON body")
	}

	switch {
	case !usernameRe.MatchString(req.Username):
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed",
			"username must be 3-32 characters of a-z, A-Z, 0-9, _ . -")
	case !emailRe.MatchString(req.Email) || len(req.Email) > 254:
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed", "invalid email address")
	case len(req.Password) < 12 || len(req.Password) > 128:
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed",
			"password must be 12-128 characters")
	case weakPassword(req.Password, req.Username, req.Email):
		return errorJSON(c, fiber.StatusUnprocessableEntity, "weak_password",
			"password is too common or based on your name/email — choose a stronger one")
	}

	count, err := h.store.CountUsers(c.Context())
	if err != nil {
		return err
	}
	// Account role + admission rules:
	//   - the very first account bootstraps the instance admin;
	//   - a valid invite grants the role it carries (and is consumed);
	//   - otherwise registration is allowed only when open registration is on.
	role := "member"
	var consumeInvite string
	switch {
	case count == 0:
		role = "admin"
	case req.InviteCode != "":
		r, err := h.store.PeekInvite(c.Context(), req.InviteCode)
		if errors.Is(err, store.ErrNotFound) {
			return errorJSON(c, fiber.StatusUnprocessableEntity, "invalid_invite",
				"this invite is invalid, already used, or expired")
		}
		if err != nil {
			return err
		}
		role = r
		consumeInvite = req.InviteCode
	case !h.cfg.OpenRegistration:
		return errorJSON(c, fiber.StatusForbidden, "registration_closed",
			"registration is invite-only — ask an admin for an invite link")
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return err
	}

	orgID, err := h.store.EnsureDefaultOrg(c.Context(), h.cfg.OrgName)
	if err != nil {
		return err
	}

	u, err := h.store.CreateUser(c.Context(), orgID, req.Username, req.Email, hash, role)
	if errors.Is(err, store.ErrConflict) {
		return errorJSON(c, fiber.StatusConflict, "already_exists", "username or email already taken")
	}
	if err != nil {
		return err
	}

	if consumeInvite != "" {
		// Best effort: if it was raced to "used" between peek and now, the
		// account still stands (a duplicate-use is harmless and rare).
		if err := h.store.MarkInviteUsed(c.Context(), consumeInvite, u.ID); err != nil {
			slog.Warn("register: invite already consumed", "code", consumeInvite, "err", err)
		}
	}

	return c.Status(fiber.StatusCreated).JSON(userJSON(u, true))
}

// POST /api/v1/auth/login
func (h *handlers) login(c *fiber.Ctx) error {
	var req struct {
		Username string     `json:"username"`
		Password string     `json:"password"`
		Device   deviceInfo `json:"device"`
	}
	if err := c.BodyParser(&req); err != nil {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed", "invalid JSON body")
	}
	if msg := req.Device.validate(); msg != "" {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed", msg)
	}

	u, err := h.store.GetUserByLogin(c.Context(), req.Username)
	if errors.Is(err, store.ErrNotFound) {
		// Burn the same CPU as a real verification before answering.
		_, _ = auth.VerifyPassword(req.Password, dummyHash)
		return errorJSON(c, fiber.StatusUnauthorized, "invalid_credentials", "invalid username or password")
	}
	if err != nil {
		return err
	}

	ok, err := auth.VerifyPassword(req.Password, u.PasswordHash)
	if err != nil {
		return err
	}
	if !ok {
		return errorJSON(c, fiber.StatusUnauthorized, "invalid_credentials", "invalid username or password")
	}
	if u.BannedAt != nil {
		return errorJSON(c, fiber.StatusForbidden, "banned", "this account is banned")
	}

	return h.issueTokens(c, u, req.Device)
}

// POST /api/v1/auth/refresh
func (h *handlers) refresh(c *fiber.Ctx) error {
	var req struct {
		RefreshToken string `json:"refresh_token"`
		DeviceID     string `json:"device_id"`
	}
	if err := c.BodyParser(&req); err != nil || req.RefreshToken == "" {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed", "invalid JSON body")
	}

	rt, err := h.store.GetRefreshToken(c.Context(), auth.HashRefreshToken(req.RefreshToken))
	if errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusUnauthorized, "invalid_refresh_token", "refresh token is not valid")
	}
	if err != nil {
		return err
	}

	if rt.DeviceID != req.DeviceID || time.Now().After(rt.ExpiresAt) {
		// Device mismatch smells like token theft; expired is just stale.
		// Either way the token is dead.
		_ = h.store.DeleteRefreshToken(c.Context(), rt.UserID, rt.DeviceID)
		return errorJSON(c, fiber.StatusUnauthorized, "invalid_refresh_token", "refresh token is not valid")
	}

	u, err := h.store.GetUserByID(c.Context(), rt.UserID)
	if err != nil {
		return err
	}
	if u.BannedAt != nil {
		_ = h.store.DeleteRefreshToken(c.Context(), rt.UserID, rt.DeviceID)
		return errorJSON(c, fiber.StatusForbidden, "banned", "this account is banned")
	}

	// Rotation: SaveRefreshToken upserts on (user_id, device_id), so issuing
	// the new token invalidates the presented one. Keep name/platform as-is.
	return h.issueTokens(c, u, deviceInfo{DeviceID: rt.DeviceID})
}

// POST /api/v1/auth/logout  (authenticated)
func (h *handlers) logout(c *fiber.Ctx) error {
	claims := mustClaims(c)
	if err := h.store.DeleteRefreshToken(c.Context(), claims.UserID, claims.DeviceID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// issueTokens creates an access/refresh pair and persists the refresh token.
func (h *handlers) issueTokens(c *fiber.Ctx, u store.User, device deviceInfo) error {
	accessToken, err := auth.NewAccessToken([]byte(h.cfg.JWTSecret), auth.Claims{
		UserID:   u.ID,
		DeviceID: device.DeviceID,
		Role:     u.Role,
	})
	if err != nil {
		return err
	}

	refreshToken, refreshHash, err := auth.NewRefreshToken()
	if err != nil {
		return err
	}
	if err := h.store.SaveRefreshToken(c.Context(), u.ID, device.DeviceID,
		device.Name, device.Platform, refreshHash,
		time.Now().Add(auth.RefreshTokenTTL)); err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    int(auth.AccessTokenTTL.Seconds()),
	})
}
