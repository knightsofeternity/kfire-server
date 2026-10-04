// Package epic talks to the Epic Games services the Epic launcher uses, with a
// member's own token. Epic publishes no API for libraries or playtime: these
// endpoints and the launcher's client credentials come from the open-source
// launcher Legendary (verified on 2026-10-04). Credentials are configured per
// instance (KFIRE_EPIC_CLIENT_ID / KFIRE_EPIC_CLIENT_SECRET), never shipped.
package epic

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
	// ErrInvalidGrant is Epic refusing a code (expired, already used) or a
	// refresh token (revoked, expired): the member must link again.
	ErrInvalidGrant = errors.New("epic: code or refresh token refused")
	// ErrClientRejected is Epic refusing the launcher credentials themselves:
	// the admin must update KFIRE_EPIC_CLIENT_ID / KFIRE_EPIC_CLIENT_SECRET.
	ErrClientRejected = errors.New("epic: launcher client credentials refused")
)

type Connector struct {
	HTTP         *http.Client
	ClientID     string
	clientSecret string
	AuthBase     string // https://account-public-service-prod03.ol.epicgames.com/account/api/oauth
	LibraryBase  string // https://library-service.live.use1a.on.epicgames.com/library/api/public
	CatalogBase  string // https://catalog-public-service-prod06.ol.epicgames.com/catalog/api/shared
}

func New(clientID, clientSecret string) *Connector {
	return &Connector{
		HTTP:         &http.Client{Timeout: 20 * time.Second},
		ClientID:     clientID,
		clientSecret: clientSecret,
		AuthBase:     "https://account-public-service-prod03.ol.epicgames.com/account/api/oauth",
		LibraryBase:  "https://library-service.live.use1a.on.epicgames.com/library/api/public",
		CatalogBase:  "https://catalog-public-service-prod06.ol.epicgames.com/catalog/api/shared",
	}
}

// Enabled reports whether the instance configured the launcher credentials.
func (c *Connector) Enabled() bool { return c.ClientID != "" && c.clientSecret != "" }

// LoginURL is the Epic sign-in page that ends on a JSON carrying the member's
// one-time authorizationCode.
func (c *Connector) LoginURL() string {
	redirect := "https://www.epicgames.com/id/api/redirect?clientId=" + c.ClientID + "&responseType=code"
	return "https://www.epicgames.com/id/login?redirectUrl=" + url.QueryEscape(redirect)
}

// get sends an authenticated GET and decodes the JSON answer into dst.
func (c *Connector) get(ctx context.Context, rawURL, token string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "bearer "+token)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrInvalidGrant
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("epic: GET %s: HTTP %d: %s", rawURL, resp.StatusCode, body)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}
