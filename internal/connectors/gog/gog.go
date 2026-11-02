// Package gog talks to the GOG services the GOG Galaxy client uses, with a
// member's own token. GOG publishes no API for libraries or playtime: these
// endpoints and Galaxy's client credentials are the ones the open-source
// launchers Heroic and Comet use (checked on 2026-11-02). Credentials are
// configured per instance (KFIRE_GOG_CLIENT_ID / KFIRE_GOG_CLIENT_SECRET),
// never shipped.
package gog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

var (
	// ErrInvalidGrant is GOG refusing a code (expired, already used) or a
	// refresh token (revoked): the member must link again.
	ErrInvalidGrant = errors.New("gog: code or refresh token refused")
	// ErrClientRejected is GOG refusing the Galaxy credentials themselves:
	// the admin must update KFIRE_GOG_CLIENT_ID / KFIRE_GOG_CLIENT_SECRET.
	ErrClientRejected = errors.New("gog: Galaxy client credentials refused")
)

// Redirect is the only redirect GOG accepts for the Galaxy client: a blank
// page whose address carries the code, which the member pastes back.
const Redirect = "https://embed.gog.com/on_login_success?origin=client"

type Connector struct {
	HTTP         *http.Client
	ClientID     string
	clientSecret string
	AuthBase     string // https://auth.gog.com
	EmbedBase    string // https://embed.gog.com
	LibraryBase  string // https://galaxy-library.gog.com
	GameplayBase string // https://gameplay.gog.com
	ProductsBase string // https://api.gog.com
}

func New(clientID, clientSecret string) *Connector {
	return &Connector{
		HTTP:         &http.Client{Timeout: 20 * time.Second},
		ClientID:     clientID,
		clientSecret: clientSecret,
		AuthBase:     "https://auth.gog.com",
		EmbedBase:    "https://embed.gog.com",
		LibraryBase:  "https://galaxy-library.gog.com",
		GameplayBase: "https://gameplay.gog.com",
		ProductsBase: "https://api.gog.com",
	}
}

// Enabled reports whether the instance configured the Galaxy credentials.
func (c *Connector) Enabled() bool { return c.ClientID != "" && c.clientSecret != "" }

// LoginURL is the GOG sign-in page that ends on Redirect with ?code=.
func (c *Connector) LoginURL() string {
	q := url.Values{"client_id": {c.ClientID}, "redirect_uri": {Redirect},
		"response_type": {"code"}, "layout": {"client2"}}
	return c.AuthBase + "/auth?" + q.Encode()
}

// errNotFound is a 404, which the gameplay service answers for a game it never
// saw a session of.
var errNotFound = errors.New("gog: not found")

// get sends a GET, authenticated when token is set, and decodes the JSON
// answer into dst.
func (c *Connector) get(ctx context.Context, rawURL, token string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return ErrInvalidGrant
	case resp.StatusCode == http.StatusNotFound:
		return errNotFound
	case resp.StatusCode >= 300:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("gog: GET %s: HTTP %d: %s", rawURL, resp.StatusCode, body)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}
