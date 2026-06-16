package api

import (
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

// TestAPIKeyCanInvite locks in the fail-closed behaviour the invite gate relies
// on: only an explicit boolean true grants permission. A missing or wrongly
// typed local must deny.
func TestAPIKeyCanInvite(t *testing.T) {
	app := fiber.New()
	cases := []struct {
		name  string
		set   bool
		value any
		want  bool
	}{
		{"unset", false, nil, false},
		{"false", true, false, false},
		{"true", true, true, true},
		{"non-bool", true, "yes", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			defer app.ReleaseCtx(c)
			if tc.set {
				c.Locals(localsAPIKeyCanInvite, tc.value)
			}
			if got := apiKeyCanInvite(c); got != tc.want {
				t.Errorf("apiKeyCanInvite = %v, want %v", got, tc.want)
			}
		})
	}
}
