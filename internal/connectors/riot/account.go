package riot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Account is one Riot account's identity.
type Account struct {
	PUUID    string `json:"puuid"`
	GameName string `json:"gameName"`
	TagLine  string `json:"tagLine"`
}

// RiotID renders the display form, "Name#TAG".
func (a Account) RiotID() string { return a.GameName + "#" + a.TagLine }

// APIError is a non-200 answer from Riot. Callers branch on Status: 404 is a
// normal, expected answer on several routes, 429 means back off.
type APIError struct {
	Status int
	Path   string
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("riot: %s: status %d: %s", e.Path, e.Status, e.Body)
}

// NotFound reports whether err is a Riot 404.
func NotFound(err error) bool {
	var e *APIError
	if ok := asAPIError(err, &e); !ok {
		return false
	}
	return e.Status == http.StatusNotFound
}

// get performs an authenticated GET against one Riot host and decodes the body
// into out. host is either a platform (euw1) or a cluster (europe).
func (c *Connector) get(ctx context.Context, host, path string, out any) error {
	endpoint := fmt.Sprintf(c.APIHostTmpl, host) + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Riot-Token", c.APIKey)

	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return &APIError{Status: res.StatusCode, Path: path, Body: string(body)}
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// AccountByPUUID resolves a PUUID to its current Riot ID.
func (c *Connector) AccountByPUUID(ctx context.Context, puuid string) (Account, error) {
	var acc Account
	err := c.get(ctx, accountCluster, "/riot/account/v1/accounts/by-puuid/"+url.PathEscape(puuid), &acc)
	return acc, err
}

// ActiveRegion returns the member's active League platform, e.g. "euw1".
func (c *Connector) ActiveRegion(ctx context.Context, puuid string) (string, error) {
	var out struct {
		Region string `json:"region"`
	}
	path := "/riot/account/v1/region/by-game/lol/by-puuid/" + url.PathEscape(puuid)
	if err := c.get(ctx, accountCluster, path, &out); err != nil {
		return "", err
	}
	if out.Region == "" {
		return "", fmt.Errorf("riot: empty active region")
	}
	return out.Region, nil
}
