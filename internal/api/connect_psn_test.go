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

// Every PlayStation route requires a bearer token.
func TestPsnRoutesRequireAuth(t *testing.T) {
	h := &handlers{cfg: &config.Config{JWTSecret: "test-secret"}}
	app := fiber.New()
	v1 := app.Group("/api/v1")
	v1.Get("/connect/psn", h.requireAuth, h.psnStatus)
	v1.Post("/connect/psn", h.requireAuth, h.connectPsn)
	v1.Post("/connect/psn/sync", h.requireAuth, h.syncPsn)
	v1.Delete("/connect/psn", h.requireAuth, h.disconnectPsn)
	for _, tc := range [][2]string{
		{"GET", "/api/v1/connect/psn"}, {"POST", "/api/v1/connect/psn"},
		{"POST", "/api/v1/connect/psn/sync"}, {"DELETE", "/api/v1/connect/psn"},
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

// An instance without the connector answers connector_disabled, and bad input
// is refused before PlayStation is ever asked.
func TestPsnWithoutConnector(t *testing.T) {
	h := &handlers{cfg: &config.Config{JWTSecret: "test-secret"}}
	app := fiber.New()
	asMember := func(c *fiber.Ctx) error {
		c.Locals(claimsKey, auth.Claims{UserID: "u1", Role: "admin"})
		return c.Next()
	}
	app.Get("/psn", asMember, h.psnStatus)
	app.Post("/psn", asMember, h.connectPsn)
	app.Put("/admin/psn", asMember, h.adminSetPsnNPSSO)
	app.Get("/admin/psn", asMember, h.adminPsnStatus)

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
	cases := []struct {
		method, path, body string
		status             int
		code               string
	}{
		{"POST", "/psn", `{"online_id":"ab"}`, 400, "invalid_psn_id"},
		{"POST", "/psn", `{"online_id":"Jampyer74"}`, 501, "connector_disabled"},
		{"GET", "/psn", ``, 501, "connector_disabled"},
		{"PUT", "/admin/psn", `{"npsso":"short"}`, 400, "invalid_npsso"},
		{"PUT", "/admin/psn", `{"npsso":"` + strings.Repeat("a", 64) + `"}`, 501, "connector_disabled"},
		{"GET", "/admin/psn", ``, 501, "connector_disabled"},
	}
	for _, tc := range cases {
		status, code := call(tc.method, tc.path, tc.body)
		if status != tc.status || code != tc.code {
			t.Errorf("%s %s %s: %d %q, want %d %q", tc.method, tc.path, tc.body, status, code, tc.status, tc.code)
		}
	}
}

// The admin may paste Sony's whole page; only the value is kept.
func TestPsnAcceptsTheWholeSonyPage(t *testing.T) {
	h := &handlers{cfg: &config.Config{JWTSecret: "test-secret"}}
	app := fiber.New()
	app.Put("/admin/psn", h.adminSetPsnNPSSO)
	page := `{"npsso":"` + strings.Repeat("b", 64) + `","expires_in":5183999}`
	body, _ := json.Marshal(map[string]string{"npsso": page})
	req := httptest.NewRequest("PUT", "/admin/psn", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	// Past validation, stopped only by the missing connector.
	if resp.StatusCode != 501 {
		t.Fatalf("the whole page was refused: %d", resp.StatusCode)
	}
}
