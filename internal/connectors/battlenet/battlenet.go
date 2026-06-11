// Package battlenet links Blizzard Battle.net accounts via OAuth 2.0
// (authorization code grant) and reads the user's BattleTag.
//
// Docs: https://develop.battle.net/documentation/guides/using-oauth
package battlenet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultOAuthBase = "https://oauth.battle.net"
const defaultAPIBaseTmpl = "https://%s.api.blizzard.com"

// Token is the result of exchanging an authorization code.
type Token struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int // seconds
	Scope        string
}

// UserInfo is the linked account's identity.
type UserInfo struct {
	Sub       string // stable account id
	BattleTag string // e.g. "Player#1234"
}

// Connector talks to Battle.net OAuth. OAuthBase and APIBase are overridable for tests.
type Connector struct {
	ClientID     string
	ClientSecret string
	OAuthBase    string
	APIBase      string
	HTTP         *http.Client
}

// New returns a connector. It is disabled until both credentials are set.
func New(clientID, clientSecret string) *Connector {
	return &Connector{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		OAuthBase:    defaultOAuthBase,
		HTTP:         &http.Client{Timeout: 15 * time.Second},
	}
}

// Enabled reports whether OAuth credentials are configured.
func (c *Connector) Enabled() bool { return c.ClientID != "" && c.ClientSecret != "" }

// AuthURL builds the authorization redirect.
func (c *Connector) AuthURL(state, redirectURI string) string {
	q := url.Values{
		"response_type": {"code"},
		"client_id":     {c.ClientID},
		"redirect_uri":  {redirectURI},
		"scope":         {"openid wow.profile sc2.profile d3.profile"},
		"state":         {state},
	}
	return c.OAuthBase + "/authorize?" + q.Encode()
}

// ExchangeCode swaps an authorization code for tokens.
func (c *Connector) ExchangeCode(ctx context.Context, code, redirectURI string) (Token, error) {
	form := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {redirectURI},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.OAuthBase+"/token", strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.ClientID, c.ClientSecret)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("battlenet token request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return Token{}, fmt.Errorf("battlenet token: HTTP %d: %s", resp.StatusCode, body)
	}

	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Token{}, fmt.Errorf("battlenet token decode: %w", err)
	}
	if payload.AccessToken == "" {
		return Token{}, errors.New("battlenet token: empty access_token")
	}
	return Token{
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		ExpiresIn:    payload.ExpiresIn,
		Scope:        payload.Scope,
	}, nil
}

// GetUserInfo returns the account's id and BattleTag.
func (c *Connector) GetUserInfo(ctx context.Context, accessToken string) (UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.OAuthBase+"/oauth/userinfo", nil)
	if err != nil {
		return UserInfo{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return UserInfo{}, fmt.Errorf("battlenet userinfo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return UserInfo{}, fmt.Errorf("battlenet userinfo: HTTP %d", resp.StatusCode)
	}

	var payload struct {
		Sub       string `json:"sub"`
		ID        int64  `json:"id"`
		BattleTag string `json:"battletag"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return UserInfo{}, fmt.Errorf("battlenet userinfo decode: %w", err)
	}
	sub := payload.Sub
	if sub == "" && payload.ID != 0 {
		sub = fmt.Sprintf("%d", payload.ID)
	}
	if sub == "" {
		return UserInfo{}, errors.New("battlenet userinfo: no account id")
	}
	return UserInfo{Sub: sub, BattleTag: payload.BattleTag}, nil
}
