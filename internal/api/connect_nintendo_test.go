package api

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/auth"
	"github.com/knightsofeternity/kfire-server/internal/config"
)

func TestNintendoRoutesRequireAuth(t *testing.T) {
	h := &handlers{cfg: &config.Config{JWTSecret: "test-secret"}}
	app := fiber.New()
	v1 := app.Group("/api/v1")
	v1.Get("/connect/nintendo", h.requireAuth, h.nintendoStatus)
	v1.Post("/connect/nintendo", h.requireAuth, h.connectNintendo)
	v1.Post("/connect/nintendo/sync", h.requireAuth, h.syncNintendo)
	v1.Delete("/connect/nintendo", h.requireAuth, h.disconnectNintendo)
	for _, tc := range [][2]string{
		{"GET", "/api/v1/connect/nintendo"}, {"POST", "/api/v1/connect/nintendo"},
		{"POST", "/api/v1/connect/nintendo/sync"}, {"DELETE", "/api/v1/connect/nintendo"},
	} {
		resp, err := app.Test(httptest.NewRequest(tc[0], tc[1], nil))
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Errorf("%s %s without a token: %d, want 401", tc[0], tc[1], resp.StatusCode)
		}
	}
}

func TestNintendoWithoutConnector(t *testing.T) {
	h := &handlers{cfg: &config.Config{JWTSecret: "test-secret"}}
	app := fiber.New()
	as := func(c *fiber.Ctx) error {
		c.Locals(claimsKey, auth.Claims{UserID: "u1", Role: "admin"})
		return c.Next()
	}
	app.Get("/n", as, h.nintendoStatus)
	app.Post("/n", as, h.connectNintendo)
	app.Get("/admin/n", as, h.adminNintendoStatus)
	app.Post("/admin/n/login", as, h.adminNintendoStartLogin)
	app.Put("/admin/n", as, h.adminNintendoFinishLogin)
	call := func(method, path, body string) (int, string) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(resp.Body)
		var out struct{ Code string }
		json.Unmarshal(raw, &out)
		return resp.StatusCode, out.Code
	}
	for _, tc := range []struct {
		method, path, body string
		status             int
		code               string
	}{
		{"POST", "/n", `{"friend_code":"Ouranos"}`, 400, "invalid_friend_code"},
		{"POST", "/n", `{"friend_code":"SW-0478-9405-4990"}`, 501, "connector_disabled"},
		{"GET", "/n", ``, 501, "connector_disabled"},
		{"GET", "/admin/n", ``, 501, "connector_disabled"},
		{"POST", "/admin/n/login", ``, 501, "connector_disabled"},
		{"PUT", "/admin/n", `{"link":"https://example.com"}`, 400, "invalid_link"},
		{"PUT", "/admin/n", `{"link":"npf71b963c1b7b6d119://auth#session_token_code=c&state=s"}`, 501, "connector_disabled"},
	} {
		if status, code := call(tc.method, tc.path, tc.body); status != tc.status || code != tc.code {
			t.Errorf("%s %s %s: %d %q, want %d %q", tc.method, tc.path, tc.body, status, code, tc.status, tc.code)
		}
	}
}
