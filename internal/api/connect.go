package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

// stateTTL bounds how long a Steam link flow may take.
const stateTTL = 10 * time.Minute

// signState returns an HMAC-signed "<userID>.<expiryUnix>.<sig>" token. It ties
// the OpenID callback (an unauthenticated browser redirect) back to the user
// who started the flow.
func signState(secret []byte, userID string) string {
	exp := time.Now().Add(stateTTL).Unix()
	msg := fmt.Sprintf("%s.%d", userID, exp)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(msg))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return msg + "." + sig
}

func verifyState(secret []byte, token string) (string, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", false
	}
	userID, expStr, sig := parts[0], parts[1], parts[2]
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(userID + "." + expStr))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return "", false
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return "", false
	}
	return userID, true
}

// GET /api/v1/connect/steam  (authenticated)
//
// Returns the "Sign in through Steam" URL. The SPA navigates the browser to it.
func (h *handlers) connectSteamStart(c *fiber.Ctx) error {
	if h.steam == nil || !h.steam.Enabled() {
		return errorJSON(c, fiber.StatusNotImplemented, "connector_disabled",
			"the Steam connector is not configured on this instance")
	}
	state := signState([]byte(h.cfg.JWTSecret), mustClaims(c).UserID)
	returnTo := fmt.Sprintf("%s/api/v1/connect/steam/callback?state=%s", h.cfg.PublicURL, state)
	url := h.steam.AuthURL(returnTo, h.cfg.PublicURL)
	return c.JSON(fiber.Map{"url": url})
}

// GET /api/v1/connect/steam/callback  (public — browser redirect from Steam)
func (h *handlers) connectSteamCallback(c *fiber.Ctx) error {
	if h.steam == nil || !h.steam.Enabled() {
		return c.Redirect("/account?steam=error")
	}

	userID, ok := verifyState([]byte(h.cfg.JWTSecret), c.Query("state"))
	if !ok {
		return c.Redirect("/account?steam=expired")
	}

	// Fiber's QueryArgs hold the openid.* params Steam appended.
	params := callbackParams(c)
	steamID, err := h.steam.VerifyCallback(c.Context(), params)
	if err != nil {
		return c.Redirect("/account?steam=denied")
	}

	// A Steam account may belong to only one KFIRE member.
	taken, err := h.store.ProviderLinkedToOther(c.Context(), "steam", steamID, userID)
	if err != nil {
		return err
	}
	if taken {
		return c.Redirect("/account?steam=conflict")
	}

	account := store.LinkedAccount{Provider: "steam", ProviderUserID: steamID}
	if player, err := h.steam.ResolvePlayer(c.Context(), steamID); err == nil {
		account.DisplayName = strPtr(player.PersonaName)
		account.AvatarURL = strPtr(player.AvatarURL)
		account.ProfileURL = strPtr(player.ProfileURL)
	}
	if err := h.store.UpsertLinkedAccount(c.Context(), userID, account); err != nil {
		return err
	}
	return c.Redirect("/account?steam=linked")
}

// DELETE /api/v1/connect/steam  (authenticated)
func (h *handlers) disconnectSteam(c *fiber.Ctx) error {
	err := h.store.DeleteLinkedAccount(c.Context(), mustClaims(c).UserID, "steam")
	if errors.Is(err, store.ErrNotFound) {
		return errorJSON(c, fiber.StatusNotFound, "not_linked", "no Steam account linked")
	}
	if err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func connectionJSON(a store.LinkedAccount) fiber.Map {
	m := fiber.Map{
		"provider":         a.Provider,
		"provider_user_id": a.ProviderUserID,
		"linked_at":        a.CreatedAt.UTC(),
	}
	if a.DisplayName != nil {
		m["display_name"] = *a.DisplayName
	}
	if a.AvatarURL != nil {
		m["avatar_url"] = *a.AvatarURL
	}
	if a.ProfileURL != nil {
		m["profile_url"] = *a.ProfileURL
	}
	return m
}

// callbackParams copies the request query into url.Values for the verifier.
func callbackParams(c *fiber.Ctx) url.Values {
	out := url.Values{}
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		out.Add(string(k), string(v))
	})
	return out
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
