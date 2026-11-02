package gog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Tokens is what a code or refresh exchange gives. The access token lives
// 1 h. The refresh token did not rotate on the 2026-11-02 probe, but GOG may
// hand back a new one: the caller always keeps the latest.
type Tokens struct {
	Access  string
	Refresh string
	UserID  string // the Galaxy user id, used in every per-user URL
}

// Exchange turns the member's one-time code into tokens.
func (c *Connector) Exchange(ctx context.Context, code string) (Tokens, error) {
	return c.token(ctx, c.ClientID, c.clientSecret,
		url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {Redirect}})
}

// Refresh renews the tokens.
func (c *Connector) Refresh(ctx context.Context, refresh string) (Tokens, error) {
	return c.token(ctx, c.ClientID, c.clientSecret,
		url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}})
}

// GameToken trades the member's Galaxy refresh token for an access token of
// one game's own Galaxy client, which its achievements need (Comet does the
// same when a game starts). without_new_session leaves the member's Galaxy
// session, and so its refresh token, as it was; the refresh token this answer
// carries is not kept.
func (c *Connector) GameToken(ctx context.Context, gameClientID, gameClientSecret, refresh string) (string, error) {
	tok, err := c.token(ctx, gameClientID, gameClientSecret,
		url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "without_new_session": {"1"}})
	return tok.Access, err
}

// token calls GOG's token endpoint, which takes everything in the query
// string of a GET (as Galaxy does).
func (c *Connector) token(ctx context.Context, clientID, clientSecret string, q url.Values) (Tokens, error) {
	q.Set("client_id", clientID)
	q.Set("client_secret", clientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.AuthBase+"/token?"+q.Encode(), nil)
	if err != nil {
		return Tokens{}, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Tokens{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		_ = json.Unmarshal(raw, &e)
		switch {
		case strings.Contains(e.Error, "invalid_client"):
			return Tokens{}, ErrClientRejected
		case resp.StatusCode == http.StatusBadRequest, resp.StatusCode == http.StatusUnauthorized:
			return Tokens{}, ErrInvalidGrant
		}
		return Tokens{}, fmt.Errorf("gog: token: HTTP %d %s", resp.StatusCode, e.Error)
	}
	var body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		UserID       string `json:"user_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Tokens{}, err
	}
	if body.AccessToken == "" || body.UserID == "" {
		return Tokens{}, fmt.Errorf("gog: token: answer without access token or user id")
	}
	return Tokens{Access: body.AccessToken, Refresh: body.RefreshToken, UserID: body.UserID}, nil
}
