package api

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/config"
	"github.com/knightsofeternity/kfire-server/internal/connectors/riot"
)

// TestRiotRoutesMountedAndProtected proves the four Riot routes are actually
// wired and all require a bearer token. Riot has no RSO application for this
// product, so there is no OAuth flow and no public callback: linking is a
// single authenticated POST from a typed Riot ID.
//
// The package has no fixture that mounts the full api.Register (it needs a
// live database for the store), so this test mounts only the Riot routes
// directly on the handlers struct, the same handler methods Register wires,
// with a disabled Riot connector (no API key). That is enough to exercise
// requireAuth's gate, which runs before any store or Riot access.
func TestRiotRoutesMountedAndProtected(t *testing.T) {
	h := &handlers{
		cfg:  &config.Config{JWTSecret: "test-secret"},
		riot: riot.New(""), // disabled: no API key configured
	}

	app := fiber.New()
	v1 := app.Group("/api/v1")
	v1.Post("/connect/riot", h.requireAuth, h.connectRiot)
	v1.Get("/connect/riot/region", h.requireAuth, h.riotRegion)
	v1.Patch("/connect/riot/region", h.requireAuth, h.updateRiotRegion)
	v1.Delete("/connect/riot", h.requireAuth, h.disconnectRiot)

	cases := []struct {
		method, path string
	}{
		{"POST", "/api/v1/connect/riot"},
		{"GET", "/api/v1/connect/riot/region"},
		{"PATCH", "/api/v1/connect/riot/region"},
		{"DELETE", "/api/v1/connect/riot"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("%s %s: request failed: %v", tc.method, tc.path, err)
		}
		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Errorf("%s %s without token: got %d, want 401", tc.method, tc.path, resp.StatusCode)
		}
	}
}
