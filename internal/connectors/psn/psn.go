// Package psn talks to the PlayStation app's API as the instance's bot
// account. Sony publishes no API: these are the endpoints its own app uses,
// verified on 2026-09-30. Every read about a member requires the bot to be
// their friend.
package psn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	clientID    = "09515159-7237-4370-9b40-3806e67c0891"
	redirectURI = "com.scee.psxandroid.scecompcall://redirect"
	scope       = "psn:mobile.v2.core psn:clientapp"
	// basicAuth is the app's public client credential, the same for everyone.
	basicAuth = "Basic MDk1MTUxNTktNzIzNy00MzcwLTliNDAtMzgwNmU2N2MwODkxOnVjUGprYTV0bnRCMktxc1A="
)

var (
	// ErrLoginRequired means Sony no longer accepts the bot's NPSSO: an admin
	// must paste a new one. Answered as a 302 to the sign-in page, code 4165.
	ErrLoginRequired = errors.New("psn: login required, the bot's NPSSO was revoked or expired")
	// ErrNotFound is an unknown online id.
	ErrNotFound = errors.New("psn: not found")
	// ErrForbidden is Sony refusing a read, almost always because the member
	// is not (or no longer) the bot's friend.
	ErrForbidden = errors.New("psn: forbidden")
)

type Connector struct {
	HTTP     *http.Client
	AuthBase string // https://ca.account.sony.com/api/authz/v3/oauth
	APIBase  string // https://m.np.playstation.com/api
	ProfBase string // https://us-prof.np.community.playstation.net/userProfile/v1/users
}

func New() *Connector {
	return &Connector{
		HTTP:     &http.Client{Timeout: 20 * time.Second},
		AuthBase: "https://ca.account.sony.com/api/authz/v3/oauth",
		APIBase:  "https://m.np.playstation.com/api",
		ProfBase: "https://us-prof.np.community.playstation.net/userProfile/v1/users",
	}
}

// Tokens are the bot's working credentials derived from its NPSSO.
type Tokens struct {
	Access         string
	AccessExpires  time.Time
	Refresh        string
	RefreshExpires time.Time
}

// do sends an authenticated request and decodes a JSON answer into dst (nil
// to ignore the body). 404 and 403 map to ErrNotFound and ErrForbidden.
func (c *Connector) do(ctx context.Context, method, url, token string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept-Language", "fr-FR")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case resp.StatusCode == http.StatusForbidden:
		return ErrForbidden
	case resp.StatusCode >= 300:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("psn: %s %s: HTTP %d: %s", method, url, resp.StatusCode, body)
	}
	if dst == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}
