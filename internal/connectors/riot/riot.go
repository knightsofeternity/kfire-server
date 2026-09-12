// Package riot links Riot Games accounts through Riot Sign On (RSO) and reads
// League of Legends data with the server's API key.
//
// Unlike the Battle.net connector, no member token is ever stored: RSO proves
// account ownership once at link time, and every later read is authenticated by
// the server-side API key alone.
//
// Docs: https://developer.riotgames.com/apis
package riot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	defaultAuthBase = "https://auth.riotgames.com"
	// defaultAPIHostTmpl takes the platform (euw1) or the cluster (europe).
	defaultAPIHostTmpl = "https://%s.api.riotgames.com"
	// accountCluster serves Account-V1. Riot lets any cluster answer for any
	// account and recommends the nearest one; KFIRE is hosted in Europe.
	accountCluster = "europe"
)

// platformToMatchCluster maps a League platform to its Match-V5 regional route.
// Note Match-V5 knows a fourth route, "sea", that Account-V1 does not.
var platformToMatchCluster = map[string]string{
	"euw1": "europe", "eun1": "europe", "tr1": "europe", "ru": "europe", "me1": "europe",
	"na1": "americas", "br1": "americas", "la1": "americas", "la2": "americas",
	"kr": "asia", "jp1": "asia",
	"oc1": "sea", "ph2": "sea", "sg2": "sea", "th2": "sea", "tw2": "sea", "vn2": "sea",
}

// MatchCluster returns the Match-V5 regional route for a platform. An unknown
// platform falls back to europe rather than failing: a wrong cluster yields an
// empty match list, which the caller already handles, while an error would lose
// the rank and mastery blocks too.
func MatchCluster(platform string) string {
	if c, ok := platformToMatchCluster[platform]; ok {
		return c
	}
	return "europe"
}

// KnownPlatforms returns every platform KFIRE accepts, sorted, for the SPA's
// region picker.
func KnownPlatforms() []string {
	out := make([]string, 0, len(platformToMatchCluster))
	for p := range platformToMatchCluster {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// Connector talks to Riot. AuthBase and APIHostTmpl are overridable for tests.
type Connector struct {
	ClientID     string
	ClientSecret string
	APIKey       string
	AuthBase     string
	APIHostTmpl  string
	HTTP         *http.Client
}

// New returns a connector. It is disabled until all three credentials are set.
func New(clientID, clientSecret, apiKey string) *Connector {
	return &Connector{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		APIKey:       apiKey,
		AuthBase:     defaultAuthBase,
		APIHostTmpl:  defaultAPIHostTmpl,
		HTTP:         &http.Client{Timeout: 15 * time.Second},
	}
}

// Enabled reports whether RSO credentials and the API key are configured.
func (c *Connector) Enabled() bool {
	return c.ClientID != "" && c.ClientSecret != "" && c.APIKey != ""
}

// AuthURL builds the RSO authorization redirect.
//
// Only the openid scope is requested. The cpid scope would return the player's
// active League region directly from userinfo, but it has an open defect where
// the field is sometimes absent; the Account-V1 active-region route is used
// instead and is the source of truth.
func (c *Connector) AuthURL(state, redirectURI string) string {
	q := url.Values{
		"response_type": {"code"},
		"client_id":     {c.ClientID},
		"redirect_uri":  {redirectURI},
		"scope":         {"openid"},
		"state":         {state},
	}
	return c.AuthBase + "/authorize?" + q.Encode()
}

// ExchangeCode swaps an authorization code for an access token. The token is
// used once, to read the PUUID, and is never persisted.
func (c *Connector) ExchangeCode(ctx context.Context, code, redirectURI string) (string, error) {
	form := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {redirectURI},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.AuthBase+"/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.ClientID, c.ClientSecret)

	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("riot token request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return "", fmt.Errorf("riot token: status %d: %s", res.StatusCode, body)
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("riot token decode: %w", err)
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("riot token: empty access_token")
	}
	return out.AccessToken, nil
}

// UserPUUID reads the linked account's PUUID from the RSO userinfo endpoint.
// The sub claim is the PUUID.
func (c *Connector) UserPUUID(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.AuthBase+"/userinfo", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("riot userinfo request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return "", fmt.Errorf("riot userinfo: status %d: %s", res.StatusCode, body)
	}
	var out struct {
		Sub string `json:"sub"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("riot userinfo decode: %w", err)
	}
	if out.Sub == "" {
		return "", fmt.Errorf("riot userinfo: empty sub")
	}
	return out.Sub, nil
}

// asAPIError is errors.As specialised to *APIError.
func asAPIError(err error, target **APIError) bool { return errors.As(err, target) }
