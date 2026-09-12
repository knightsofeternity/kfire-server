package riot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const defaultDataDragonBase = "https://ddragon.leagueoflegends.com"

// ddTTL bounds how long a champion table is reused before Riot's current
// version is checked again. Champions change at patch cadence, not hourly.
const ddTTL = 24 * time.Hour

// champion is one Data Dragon champion entry, reduced to what we display.
type champion struct {
	name string
	key  string // the image basename, e.g. "Jhin"
}

// DataDragon resolves numeric champion ids to names and icon URLs, caching the
// whole table for a day. Data Dragon is a static CDN and needs no API key.
type DataDragon struct {
	Base string
	HTTP *http.Client

	mu        sync.Mutex
	version   string
	champions map[int]champion
	fetchedAt time.Time
}

// NewDataDragon returns a Data Dragon client with an empty cache.
func NewDataDragon() *DataDragon {
	return &DataDragon{
		Base: defaultDataDragonBase,
		HTTP: &http.Client{Timeout: 15 * time.Second},
	}
}

// Champion returns the champion's display name and icon URL. An id absent from
// the table yields two empty strings and no error: the SPA then shows the bare
// id, which is better than losing the whole mastery block.
func (d *DataDragon) Champion(ctx context.Context, id int) (string, string, error) {
	if err := d.ensure(ctx); err != nil {
		return "", "", err
	}
	d.mu.Lock()
	c, ok := d.champions[id]
	version := d.version
	d.mu.Unlock()
	if !ok {
		return "", "", nil
	}
	return c.name, fmt.Sprintf("%s/cdn/%s/img/champion/%s.png", d.Base, version, c.key), nil
}

// ensure loads the champion table if the cache is empty or stale.
func (d *DataDragon) ensure(ctx context.Context) error {
	d.mu.Lock()
	fresh := d.champions != nil && time.Since(d.fetchedAt) < ddTTL
	d.mu.Unlock()
	if fresh {
		return nil
	}

	version, err := d.latestVersion(ctx)
	if err != nil {
		return err
	}
	table, err := d.championTable(ctx, version)
	if err != nil {
		return err
	}

	d.mu.Lock()
	d.version, d.champions, d.fetchedAt = version, table, time.Now()
	d.mu.Unlock()
	return nil
}

func (d *DataDragon) latestVersion(ctx context.Context) (string, error) {
	var versions []string
	if err := d.getJSON(ctx, "/api/versions.json", &versions); err != nil {
		return "", err
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("data dragon: empty version list")
	}
	return versions[0], nil
}

func (d *DataDragon) championTable(ctx context.Context, version string) (map[int]champion, error) {
	var raw struct {
		Data map[string]struct {
			Key  string `json:"key"`
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	path := "/cdn/" + version + "/data/en_US/champion.json"
	if err := d.getJSON(ctx, path, &raw); err != nil {
		return nil, err
	}
	out := make(map[int]champion, len(raw.Data))
	for _, c := range raw.Data {
		id, err := strconv.Atoi(c.Key)
		if err != nil {
			continue
		}
		out[id] = champion{name: c.Name, key: c.ID}
	}
	return out, nil
}

func (d *DataDragon) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.Base+path, nil)
	if err != nil {
		return err
	}
	res, err := d.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("data dragon request %s: %w", path, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 256))
		return fmt.Errorf("data dragon: %s: status %d: %s", path, res.StatusCode, body)
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("data dragon decode %s: %w", path, err)
	}
	return nil
}
