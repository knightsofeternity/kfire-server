// Package steam implements account linking and profile lookups via Steam.
//
// Linking uses Steam's OpenID 2.0 ("Sign in through Steam"), which yields the
// user's 64-bit SteamID — no per-user secret, so nothing needs encrypting.
// Profile and (later) library/achievement reads use the server's Steam Web API
// key. Docs: https://steamcommunity.com/dev and https://partner.steamgames.com/doc/webapi
package steam

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	defaultLoginBase = "https://steamcommunity.com/openid/login"
	defaultAPIBase   = "https://api.steampowered.com"
	openidNS         = "http://specs.openid.net/auth/2.0"
	identifierSelect = "http://specs.openid.net/auth/2.0/identifier_select"
)

// claimedIDRe extracts the SteamID64 from the OpenID claimed identifier.
var claimedIDRe = regexp.MustCompile(`^https://steamcommunity\.com/openid/id/(\d{17})$`)

// ErrAssertionInvalid means Steam did not confirm the OpenID response (forged
// or tampered callback).
var ErrAssertionInvalid = errors.New("steam openid assertion not valid")

// Player is the subset of a Steam profile we display.
type Player struct {
	SteamID     string
	PersonaName string
	AvatarURL   string
	ProfileURL  string
}

// Connector talks to Steam. The base URLs are overridable for tests.
type Connector struct {
	APIKey    string
	LoginBase string
	APIBase   string
	HTTP      *http.Client
}

// New returns a Steam connector using the production endpoints.
func New(apiKey string) *Connector {
	return &Connector{
		APIKey:    apiKey,
		LoginBase: defaultLoginBase,
		APIBase:   defaultAPIBase,
		HTTP:      &http.Client{Timeout: 15 * time.Second},
	}
}

// Enabled reports whether a Web API key is configured.
func (c *Connector) Enabled() bool { return c.APIKey != "" }

// AuthURL builds the redirect that starts the "Sign in through Steam" flow.
// returnTo is the absolute callback URL (it may carry its own query, e.g. a
// signed state); realm is its scheme+host.
func (c *Connector) AuthURL(returnTo, realm string) string {
	q := url.Values{
		"openid.ns":         {openidNS},
		"openid.mode":       {"checkid_setup"},
		"openid.return_to":  {returnTo},
		"openid.realm":      {realm},
		"openid.identity":   {identifierSelect},
		"openid.claimed_id": {identifierSelect},
	}
	return c.LoginBase + "?" + q.Encode()
}

// VerifyCallback validates the OpenID response with Steam and returns the
// authenticated SteamID64. params are the query values Steam appended to the
// callback URL.
func (c *Connector) VerifyCallback(ctx context.Context, params url.Values) (string, error) {
	claimed := params.Get("openid.claimed_id")
	m := claimedIDRe.FindStringSubmatch(claimed)
	if m == nil {
		return "", fmt.Errorf("%w: unexpected claimed_id %q", ErrAssertionInvalid, claimed)
	}

	// Echo every openid.* parameter back with mode=check_authentication.
	verify := url.Values{}
	for k, v := range params {
		if strings.HasPrefix(k, "openid.") {
			verify[k] = v
		}
	}
	verify.Set("openid.mode", "check_authentication")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.LoginBase,
		strings.NewReader(verify.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("steam verify request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	// Steam answers a tiny key:value body; we require is_valid:true.
	if !strings.Contains(string(body), "is_valid:true") {
		return "", ErrAssertionInvalid
	}
	return m[1], nil
}

// ResolvePlayer fetches a player's persona via GetPlayerSummaries.
func (c *Connector) ResolvePlayer(ctx context.Context, steamID string) (Player, error) {
	q := url.Values{"key": {c.APIKey}, "steamids": {steamID}}
	endpoint := fmt.Sprintf("%s/ISteamUser/GetPlayerSummaries/v2/?%s", c.APIBase, q.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Player{}, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Player{}, fmt.Errorf("steam summaries request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Player{}, fmt.Errorf("steam summaries: HTTP %d", resp.StatusCode)
	}

	var payload struct {
		Response struct {
			Players []struct {
				SteamID     string `json:"steamid"`
				PersonaName string `json:"personaname"`
				AvatarFull  string `json:"avatarfull"`
				ProfileURL  string `json:"profileurl"`
			} `json:"players"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Player{}, fmt.Errorf("steam summaries decode: %w", err)
	}
	if len(payload.Response.Players) == 0 {
		return Player{}, fmt.Errorf("steam summaries: no player for %s", steamID)
	}
	p := payload.Response.Players[0]
	return Player{
		SteamID:     p.SteamID,
		PersonaName: p.PersonaName,
		AvatarURL:   p.AvatarFull,
		ProfileURL:  p.ProfileURL,
	}, nil
}
