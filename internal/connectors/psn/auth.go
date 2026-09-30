package psn

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Exchange turns an NPSSO into tokens: authorize (302 carrying a code) then
// token. ErrLoginRequired when Sony refuses the NPSSO.
func (c *Connector) Exchange(ctx context.Context, npsso string) (Tokens, error) {
	q := url.Values{
		"access_type": {"offline"}, "client_id": {clientID}, "redirect_uri": {redirectURI},
		"response_type": {"code"}, "scope": {scope},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.AuthBase+"/authorize?"+q.Encode(), nil)
	if err != nil {
		return Tokens{}, err
	}
	req.Header.Set("Cookie", "npsso="+npsso)
	noRedirect := *c.HTTP
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := noRedirect.Do(req)
	if err != nil {
		return Tokens{}, err
	}
	resp.Body.Close()
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		return Tokens{}, fmt.Errorf("psn: authorize: bad redirect: %w", err)
	}
	params := loc.Query()
	if code := params.Get("code"); code != "" {
		return c.token(ctx, url.Values{
			"code": {code}, "redirect_uri": {redirectURI},
			"grant_type": {"authorization_code"}, "token_format": {"jwt"},
		})
	}
	if params.Get("error") == "login_required" || params.Get("error_code") == "4165" {
		return Tokens{}, ErrLoginRequired
	}
	return Tokens{}, fmt.Errorf("psn: authorize: HTTP %d, error %q", resp.StatusCode, params.Get("error"))
}

// Refresh renews the access token without the NPSSO, for as long as the
// refresh token lives (10 days, it does not rotate).
func (c *Connector) Refresh(ctx context.Context, refresh string) (Tokens, error) {
	return c.token(ctx, url.Values{
		"refresh_token": {refresh}, "grant_type": {"refresh_token"},
		"scope": {scope}, "token_format": {"jwt"},
	})
}

func (c *Connector) token(ctx context.Context, form url.Values) (Tokens, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.AuthBase+"/token", strings.NewReader(form.Encode()))
	if err != nil {
		return Tokens{}, err
	}
	req.Header.Set("Authorization", basicAuth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Tokens{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Tokens{}, fmt.Errorf("psn: token: HTTP %d", resp.StatusCode)
	}
	var body struct {
		AccessToken           string `json:"access_token"`
		ExpiresIn             int    `json:"expires_in"`
		RefreshToken          string `json:"refresh_token"`
		RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Tokens{}, err
	}
	now := time.Now()
	return Tokens{
		Access:         body.AccessToken,
		AccessExpires:  now.Add(time.Duration(body.ExpiresIn) * time.Second),
		Refresh:        body.RefreshToken,
		RefreshExpires: now.Add(time.Duration(body.RefreshTokenExpiresIn) * time.Second),
	}, nil
}

// Profile is an account's identity on PSN.
type Profile struct {
	AccountID string `json:"accountId"`
	OnlineID  string `json:"onlineId"`
}

// Me returns the bot's own identity.
func (c *Connector) Me(ctx context.Context, token string) (Profile, error) {
	return c.profile(ctx, token, "me")
}

// Lookup resolves a member's online id. ErrNotFound for an unknown one.
func (c *Connector) Lookup(ctx context.Context, token, onlineID string) (Profile, error) {
	return c.profile(ctx, token, url.PathEscape(onlineID))
}

func (c *Connector) profile(ctx context.Context, token, who string) (Profile, error) {
	var body struct {
		Profile Profile `json:"profile"`
	}
	err := c.do(ctx, http.MethodGet, c.ProfBase+"/"+who+"/profile2?fields=accountId,onlineId", token, &body)
	if err != nil {
		return Profile{}, err
	}
	if body.Profile.AccountID == "" {
		return Profile{}, ErrNotFound
	}
	return body.Profile, nil
}
