package epic

import (
	"context"
	"fmt"
	"net/url"
)

// maxLibraryPages bounds pagination; a library page holds about 100 items.
const maxLibraryPages = 50

// Item is one library entry.
type Item struct {
	AppName       string `json:"appName"`
	Namespace     string `json:"namespace"`
	CatalogItemID string `json:"catalogItemId"`
}

// Library returns the member's whole library, page after page.
func (c *Connector) Library(ctx context.Context, token string) ([]Item, error) {
	var out []Item
	cursor := ""
	for page := 0; page < maxLibraryPages; page++ {
		u := c.LibraryBase + "/items?includeMetadata=true"
		if cursor != "" {
			u += "&cursor=" + url.QueryEscape(cursor)
		}
		var body struct {
			Records          []Item `json:"records"`
			ResponseMetadata struct {
				NextCursor string `json:"nextCursor"`
			} `json:"responseMetadata"`
		}
		if err := c.get(ctx, u, token, &body); err != nil {
			return nil, err
		}
		out = append(out, body.Records...)
		if body.ResponseMetadata.NextCursor == "" {
			return out, nil
		}
		cursor = body.ResponseMetadata.NextCursor
	}
	return out, nil
}

// CatalogEntry is what the catalog says about an item.
type CatalogEntry struct {
	Title      string
	Categories []string
	Image      string // DieselGameBoxTall, else Thumbnail, else the first image
}

// CatalogItem describes one item, in English: the catalog games KFIRE joins
// them to carry English names ("Édition Jeu de l'année" would never match).
func (c *Connector) CatalogItem(ctx context.Context, token, namespace, catalogItemID string) (CatalogEntry, error) {
	u := fmt.Sprintf("%s/namespace/%s/bulk/items?id=%s&country=US&locale=en-US",
		c.CatalogBase, url.PathEscape(namespace), url.QueryEscape(catalogItemID))
	var body map[string]struct {
		Title      string `json:"title"`
		Categories []struct {
			Path string `json:"path"`
		} `json:"categories"`
		KeyImages []struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"keyImages"`
	}
	if err := c.get(ctx, u, token, &body); err != nil {
		return CatalogEntry{}, err
	}
	raw, ok := body[catalogItemID]
	if !ok {
		return CatalogEntry{}, fmt.Errorf("epic: catalog has no %s/%s", namespace, catalogItemID)
	}
	e := CatalogEntry{Title: raw.Title}
	for _, cat := range raw.Categories {
		e.Categories = append(e.Categories, cat.Path)
	}
	for _, want := range []string{"DieselGameBoxTall", "Thumbnail"} {
		for _, img := range raw.KeyImages {
			if e.Image == "" && img.Type == want {
				e.Image = img.URL
			}
		}
	}
	if e.Image == "" && len(raw.KeyImages) > 0 {
		e.Image = raw.KeyImages[0].URL
	}
	return e, nil
}

// Playtime is the time Epic counted for one library artifact (its appName).
type Playtime struct {
	ArtifactID string `json:"artifactId"`
	TotalTime  int64  `json:"totalTime"` // seconds
}

// Playtime returns what Epic counted; only some games are counted at all.
func (c *Connector) Playtime(ctx context.Context, token, accountID string) ([]Playtime, error) {
	var out []Playtime
	err := c.get(ctx, c.LibraryBase+"/playtime/account/"+url.PathEscape(accountID)+"/all", token, &out)
	return out, err
}
