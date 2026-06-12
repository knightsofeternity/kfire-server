// Package xbox links Xbox accounts via OpenXBL (https://xbl.io) and reads
// presence (currently-playing title), gamertag and gamerscore. The OAuth
// linking flow is added separately once the OpenXBL app is configured.
package xbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultAPIBase   = "https://xbl.io"
	defaultClaimBase = "https://api.xbl.io"
)

type Connector struct {
	AppKey    string
	APIBase   string
	ClaimBase string
	HTTP      *http.Client
}

func New(appKey string) *Connector {
	return &Connector{AppKey: appKey, APIBase: defaultAPIBase, ClaimBase: defaultClaimBase, HTTP: &http.Client{Timeout: 15 * time.Second}}
}

func (c *Connector) Enabled() bool { return c.AppKey != "" }

func (c *Connector) base() string {
	if c.APIBase != "" {
		return c.APIBase
	}
	return defaultAPIBase
}

func (c *Connector) claimBase() string {
	if c.ClaimBase != "" {
		return c.ClaimBase
	}
	return defaultClaimBase
}

// AuthURL returns the OpenXBL "Sign in with Xbox" URL for our app's public key.
func (c *Connector) AuthURL(publicKey string) string {
	return c.base() + "/app/auth/" + publicKey
}

// ExchangeCode claims an OpenXBL authorization code for the member's secret key
// (used thereafter as X-Authorization for their data calls).
func (c *Connector) ExchangeCode(ctx context.Context, code, publicKey string) (string, error) {
	body, _ := json.Marshal(map[string]string{"code": code, "app_key": publicKey})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.claimBase()+"/app/claim", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openxbl claim: HTTP %d: %s", resp.StatusCode, raw)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", fmt.Errorf("openxbl claim: decode: %w", err)
	}
	// Log only the keys (the value is a secret) so we can confirm the field name.
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	slog.Info("xbox claim response", "keys", keys)
	for _, field := range []string{"app_key", "key", "token", "secret", "X-Authorization"} {
		if v, ok := obj[field].(string); ok && v != "" {
			return v, nil
		}
	}
	for _, v := range obj { // fallback: first non-empty string value
		if s, ok := v.(string); ok && s != "" {
			return s, nil
		}
	}
	return "", fmt.Errorf("openxbl claim: no key in response")
}

// Account is the linked identity.
type Account struct {
	XUID       string
	Gamertag   string
	Gamerscore int
}

// Presence is a member's current Xbox presence.
type Presence struct {
	Playing   bool
	TitleID   string
	TitleName string
}

// get performs an authenticated GET. token is the member's OpenXBL secret key,
// sent as X-Authorization. A 404 yields found=false (no error).
func (c *Connector) get(ctx context.Context, path, token string, dst any) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+path, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("X-Authorization", token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("openxbl: HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return false, err
	}
	return true, nil
}

func (c *Connector) Account(ctx context.Context, token string) (Account, error) {
	var payload struct {
		ProfileUsers []struct {
			ID       string `json:"id"`
			Settings []struct {
				ID    string `json:"id"`
				Value string `json:"value"`
			} `json:"settings"`
		} `json:"profileUsers"`
	}
	found, err := c.get(ctx, "/api/v2/account", token, &payload)
	if err != nil {
		return Account{}, err
	}
	if !found || len(payload.ProfileUsers) == 0 {
		return Account{}, fmt.Errorf("openxbl: no profile")
	}
	u := payload.ProfileUsers[0]
	a := Account{XUID: u.ID}
	for _, s := range u.Settings {
		switch s.ID {
		case "Gamertag":
			a.Gamertag = s.Value
		case "Gamerscore":
			a.Gamerscore, _ = strconv.Atoi(s.Value)
		}
	}
	return a, nil
}

func (c *Connector) Presence(ctx context.Context, token, xuid string) (*Presence, error) {
	var payload []struct {
		State   string `json:"state"`
		Devices []struct {
			Titles []struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				State string `json:"state"`
			} `json:"titles"`
		} `json:"devices"`
	}
	found, err := c.get(ctx, "/api/v2/"+xuid+"/presence", token, &payload)
	if err != nil || !found || len(payload) == 0 {
		return &Presence{}, err
	}
	for _, d := range payload[0].Devices {
		for _, ti := range d.Titles {
			if ti.State == "Active" && ti.Name != "" {
				return &Presence{Playing: true, TitleID: ti.ID, TitleName: ti.Name}, nil
			}
		}
	}
	return &Presence{}, nil
}
