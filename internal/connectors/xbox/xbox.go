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
// Auth and claim live on the API host (api.xbl.io); only data calls use xbl.io.
func (c *Connector) AuthURL(publicKey string) string {
	return c.claimBase() + "/app/auth/" + publicKey
}

// ClaimResult is the outcome of claiming an OpenXBL authorization code: the
// member's secret key (used as X-Authorization for their data calls) plus the
// identity OpenXBL returns alongside it, so no extra /account call is needed
// (that endpoint returns an empty profile for app-scoped member keys).
type ClaimResult struct {
	Key      string
	XUID     string
	Gamertag string
}

// ExchangeCode claims an OpenXBL authorization code, returning the member's key
// and identity. The claim is slow (OpenXBL runs the full Microsoft/XSTS
// exchange server-side, ~10-30s), so it uses a dedicated long timeout rather
// than the snappy c.HTTP used for data calls.
func (c *Connector) ExchangeCode(ctx context.Context, code, publicKey string) (ClaimResult, error) {
	body, _ := json.Marshal(map[string]string{"code": code, "app_key": publicKey})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.claimBase()+"/app/claim", bytes.NewReader(body))
	if err != nil {
		return ClaimResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	claimHTTP := &http.Client{Timeout: 60 * time.Second}
	resp, err := claimHTTP.Do(req)
	if err != nil {
		return ClaimResult{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK {
		return ClaimResult{}, fmt.Errorf("openxbl claim: HTTP %d: %s", resp.StatusCode, raw)
	}
	var payload struct {
		AppKey   string `json:"app_key"`
		XUID     string `json:"xuid"`
		Gamertag string `json:"gamertag"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ClaimResult{}, fmt.Errorf("openxbl claim: decode: %w", err)
	}
	if payload.AppKey == "" {
		return ClaimResult{}, fmt.Errorf("openxbl claim: no app_key in response")
	}
	slog.Info("xbox: claim ok", "xuid", payload.XUID, "gamertag", payload.Gamertag)
	return ClaimResult{Key: payload.AppKey, XUID: payload.XUID, Gamertag: payload.Gamertag}, nil
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
