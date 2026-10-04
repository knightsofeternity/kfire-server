package epic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Tokens is what a code or refresh exchange gives. The access token lives
// 36 h; the refresh token a year and rotates on every refresh.
type Tokens struct {
	Access         string
	Refresh        string
	RefreshExpires time.Time
	AccountID      string
	DisplayName    string
}

// Exchange turns the member's one-time authorizationCode into tokens.
func (c *Connector) Exchange(ctx context.Context, code string) (Tokens, error) {
	return c.token(ctx, url.Values{"grant_type": {"authorization_code"}, "code": {code}, "token_type": {"eg1"}})
}

// Refresh renews the tokens; Epic hands back a new refresh token to keep.
func (c *Connector) Refresh(ctx context.Context, refresh string) (Tokens, error) {
	return c.token(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "token_type": {"eg1"}})
}

func (c *Connector) token(ctx context.Context, form url.Values) (Tokens, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.AuthBase+"/token", strings.NewReader(form.Encode()))
	if err != nil {
		return Tokens{}, err
	}
	req.SetBasicAuth(c.ClientID, c.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Tokens{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var e struct {
			ErrorCode string `json:"errorCode"`
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		_ = json.Unmarshal(raw, &e)
		switch {
		case strings.Contains(e.ErrorCode, "invalid_client"):
			return Tokens{}, ErrClientRejected
		case resp.StatusCode == http.StatusBadRequest:
			return Tokens{}, ErrInvalidGrant
		}
		return Tokens{}, fmt.Errorf("epic: token: HTTP %d %s", resp.StatusCode, e.ErrorCode)
	}
	var body struct {
		AccessToken    string `json:"access_token"`
		RefreshToken   string `json:"refresh_token"`
		RefreshExpires int    `json:"refresh_expires"`
		AccountID      string `json:"account_id"`
		DisplayName    string `json:"displayName"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Tokens{}, err
	}
	return Tokens{
		Access: body.AccessToken, Refresh: body.RefreshToken,
		RefreshExpires: time.Now().Add(time.Duration(body.RefreshExpires) * time.Second),
		AccountID:      body.AccountID, DisplayName: body.DisplayName,
	}, nil
}
