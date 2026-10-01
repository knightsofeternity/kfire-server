// Package nintendo reads a Nintendo Switch bot's friends through an nxapi
// sidecar. Nintendo publishes no API: the Nintendo Switch Online app's API
// ("Coral") needs an "f" token computed outside Nintendo and encrypted
// requests, which nxapi handles and keeps up with app updates. This package
// only speaks the sidecar's generic call endpoint, verified on 2026-10-01.
package nintendo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var (
	// ErrSessionRevoked means Nintendo no longer accepts the bot's session
	// token: an admin must log the bot in again.
	ErrSessionRevoked = errors.New("nintendo: the bot's session token was revoked")
	// ErrNotFound is an unknown friend code or a member who is not a friend.
	ErrNotFound = errors.New("nintendo: not found")
	// ErrRateLimited is Nintendo refusing a burst of calls, friend code
	// lookups in particular (seen after a handful in a minute). Never retried
	// automatically: the nxapi terms forbid it.
	ErrRateLimited = errors.New("nintendo: rate limited")
)

// coralNotFound is Coral's "Resource not found", seen on 2026-10-01 for an
// unknown friend code and for a play log or friend that is not ours.
const coralNotFound = 9402

// Client calls the nxapi sidecar.
type Client struct {
	BaseURL string // e.g. http://nxapi:8080
	HTTP    *http.Client
}

func New(baseURL string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), HTTP: &http.Client{Timeout: 60 * time.Second}}
}

// Enabled reports whether a sidecar is configured.
func (c *Client) Enabled() bool { return c != nil && c.BaseURL != "" }

// call runs one Coral API call through the sidecar and decodes its result.
func (c *Client) call(ctx context.Context, session, url string, param, dst any) error {
	body, err := json.Marshal(map[string]any{"url": url, "parameter": param})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/znc/call", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "na "+session)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error        string `json:"error"`
			ErrorMessage string `json:"error_message"`
			Data         struct {
				Error  string `json:"error"`
				Status int    `json:"status"`
			} `json:"data"`
		}
		_ = json.Unmarshal(raw, &e)
		if e.Error == "invalid_grant" || e.Data.Error == "invalid_grant" {
			return ErrSessionRevoked
		}
		// Coral's "Resource not found" (status 9402): an unknown friend code,
		// or a member who is not, or no longer, the bot's friend.
		if e.Data.Status == coralNotFound {
			return ErrNotFound
		}
		if strings.Contains(strings.ToLower(e.ErrorMessage), "rate limit") {
			return ErrRateLimited
		}
		return fmt.Errorf("nintendo: %s: HTTP %d: %s %s", url, resp.StatusCode, e.Error, e.ErrorMessage)
	}
	var env struct {
		Status       int             `json:"status"`
		ErrorMessage string          `json:"errorMessage"`
		Result       json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("nintendo: %s: %w", url, err)
	}
	if env.Status != 0 {
		if env.Status == coralNotFound {
			return ErrNotFound
		}
		return fmt.Errorf("nintendo: %s: status %d: %s", url, env.Status, env.ErrorMessage)
	}
	if dst == nil || len(env.Result) == 0 {
		return nil
	}
	return json.Unmarshal(env.Result, dst)
}

// Game is a game as Coral describes it: in presence or in a play log.
type Game struct {
	Name           string `json:"name"`
	ImageURI       string `json:"imageUri"`
	ShopURI        string `json:"shopUri"`
	TotalPlayTime  int64  `json:"totalPlayTime"` // minutes
	FirstPlayedAt  int64  `json:"firstPlayedAt"` // Unix seconds, 0 if never
	SysDescription string `json:"sysDescription"`
}

// Friend is one of the bot's friends with their presence.
type Friend struct {
	NsaID    string `json:"nsaId"`
	Name     string `json:"name"`
	Presence struct {
		State     string `json:"state"`
		UpdatedAt int64  `json:"updatedAt"`
		Game      Game   `json:"game"`
	} `json:"presence"`
}

// Playing reports whether the friend is in a game right now.
func (f Friend) Playing() bool {
	return (f.Presence.State == "PLAYING" || f.Presence.State == "ONLINE") && f.Presence.Game.Name != ""
}

// Friends returns every friend of the bot, with presence, in one call.
func (c *Client) Friends(ctx context.Context, session string) ([]Friend, error) {
	var r struct {
		Friends []Friend `json:"friends"`
	}
	err := c.call(ctx, session, "/v4/Friend/List", map[string]any{}, &r)
	return r.Friends, err
}

// FriendRequest is a request waiting for the bot.
type FriendRequest struct {
	ID     string `json:"id"`
	Sender struct {
		NsaID string `json:"nsaId"`
		Name  string `json:"name"`
	} `json:"sender"`
}

func (c *Client) ReceivedRequests(ctx context.Context, session string) ([]FriendRequest, error) {
	var r struct {
		FriendRequests []FriendRequest `json:"friendRequests"`
	}
	err := c.call(ctx, session, "/v4/FriendRequest/Received/List", map[string]any{}, &r)
	return r.FriendRequests, err
}

func (c *Client) Accept(ctx context.Context, session, requestID string) error {
	return c.call(ctx, session, "/v3/FriendRequest/Accept", map[string]any{"id": requestID}, nil)
}

func (c *Client) DeleteFriend(ctx context.Context, session, nsaID string) error {
	return c.call(ctx, session, "/v3/Friend/Delete", map[string]any{"nsaId": nsaID}, nil)
}

// Self is the bot's own Switch identity.
type Self struct {
	NsaID      string
	Name       string
	FriendCode string
}

// Me returns the bot's identity, friend code included.
func (c *Client) Me(ctx context.Context, session string) (Self, error) {
	var r struct {
		NsaID string `json:"nsaId"`
		Name  string `json:"name"`
		Links struct {
			FriendCode struct {
				ID string `json:"id"`
			} `json:"friendCode"`
		} `json:"links"`
	}
	err := c.call(ctx, session, "/v4/User/ShowSelf", map[string]any{}, &r)
	return Self{NsaID: r.NsaID, Name: r.Name, FriendCode: r.Links.FriendCode.ID}, err
}

// User is a Switch user found by friend code.
type User struct {
	NsaID    string `json:"nsaId"`
	Name     string `json:"name"`
	ImageURI string `json:"imageUri"`
}

// LookupFriendCode resolves a normalized friend code ("0478-9405-4990").
func (c *Client) LookupFriendCode(ctx context.Context, session, code string) (User, error) {
	var u User
	err := c.call(ctx, session, "/v3/Friend/GetUserByFriendCode", map[string]any{"friendCode": code}, &u)
	if err == nil && u.NsaID == "" {
		return u, ErrNotFound
	}
	return u, err
}

// PlayLog returns a friend's play history, every game with its total time.
func (c *Client) PlayLog(ctx context.Context, session, nsaID string) ([]Game, error) {
	var games []Game
	err := c.call(ctx, session, "/v4/User/PlayLog/Show", map[string]any{"nsaId": nsaID}, &games)
	return games, err
}

var (
	shopApp    = regexp.MustCompile(`/apps/([0-9A-Fa-f]{16})(?:/|$|\?)`)
	codeDigits = regexp.MustCompile(`\d`)
)

// TitleID extracts the eShop title id from a shop URI, or "" when absent.
func TitleID(shopURI string) string {
	m := shopApp.FindStringSubmatch(shopURI)
	if m == nil {
		return ""
	}
	return strings.ToLower(m[1])
}

// NormalizeFriendCode accepts "SW-0478-9405-4990", "0478 9405 4990" or
// "047894054990" and returns "0478-9405-4990".
func NormalizeFriendCode(in string) (string, error) {
	s := strings.TrimSpace(strings.ToUpper(in))
	s = strings.TrimPrefix(s, "SW")
	for _, r := range s {
		if !(r >= '0' && r <= '9') && r != '-' && r != ' ' {
			return "", fmt.Errorf("nintendo: %q is not a friend code", in)
		}
	}
	d := strings.Join(codeDigits.FindAllString(s, -1), "")
	if len(d) != 12 {
		return "", fmt.Errorf("nintendo: %q is not a friend code", in)
	}
	return d[0:4] + "-" + d[4:8] + "-" + d[8:12], nil
}
