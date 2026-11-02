package gog

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

const (
	// maxLibraryPages bounds pagination; a page holds up to 500 entries.
	maxLibraryPages = 20
	// productsPerCall is how many ids api.gog.com/products takes at once.
	productsPerCall = 50
)

// Username is the member's GOG name.
func (c *Connector) Username(ctx context.Context, token string) (string, error) {
	var body struct {
		Username string `json:"username"`
	}
	err := c.get(ctx, c.EmbedBase+"/userData.json", token, &body)
	return body.Username, err
}

// OwnedGOGProducts returns the ids of the GOG products in the member's Galaxy
// library. That library also lists games Galaxy imported from Steam, Epic,
// Xbox and others; only platform "gog" is the member's GOG purchases, the rest
// KFIRE gets from its own connectors.
func (c *Connector) OwnedGOGProducts(ctx context.Context, token, userID string) ([]string, error) {
	var out []string
	pageToken := ""
	for page := 0; page < maxLibraryPages; page++ {
		u := c.LibraryBase + "/users/" + url.PathEscape(userID) + "/releases"
		if pageToken != "" {
			u += "?page_token=" + url.QueryEscape(pageToken)
		}
		var body struct {
			Items []struct {
				PlatformID string `json:"platform_id"`
				ExternalID string `json:"external_id"`
			} `json:"items"`
			NextPageToken string `json:"next_page_token"`
		}
		if err := c.get(ctx, u, token, &body); err != nil {
			return nil, err
		}
		for _, it := range body.Items {
			if it.PlatformID == "gog" && it.ExternalID != "" {
				out = append(out, it.ExternalID)
			}
		}
		if body.NextPageToken == "" {
			return out, nil
		}
		pageToken = body.NextPageToken
	}
	return out, nil
}

// Product is what the public catalog says about a product.
type Product struct {
	ID       string
	Title    string
	GameType string // "game", "dlc", "pack"
	Image    string // logo2x, else logo, else icon; absolute https URL
}

// Products describes products, in batches; ids the catalog no longer knows
// are simply missing from the answer.
func (c *Connector) Products(ctx context.Context, ids []string) ([]Product, error) {
	var out []Product
	for start := 0; start < len(ids); start += productsPerCall {
		end := min(start+productsPerCall, len(ids))
		var body []struct {
			ID       int64  `json:"id"`
			Title    string `json:"title"`
			GameType string `json:"game_type"`
			Images   struct {
				Logo2x string `json:"logo2x"`
				Logo   string `json:"logo"`
				Icon   string `json:"icon"`
			} `json:"images"`
		}
		u := c.ProductsBase + "/products?ids=" + url.QueryEscape(strings.Join(ids[start:end], ","))
		if err := c.get(ctx, u, "", &body); err != nil {
			return nil, err
		}
		for _, p := range body {
			img := p.Images.Logo2x
			if img == "" {
				img = p.Images.Logo
			}
			if img == "" {
				img = p.Images.Icon
			}
			if strings.HasPrefix(img, "//") {
				img = "https:" + img
			}
			out = append(out, Product{ID: strconv.FormatInt(p.ID, 10), Title: p.Title, GameType: p.GameType, Image: img})
		}
	}
	return out, nil
}

// PlayedMinutes is the total time Galaxy counted for one game, in minutes;
// 0 for a game GOG never saw a session of.
func (c *Connector) PlayedMinutes(ctx context.Context, token, userID, productID string) (int64, error) {
	var body struct {
		TimeSum int64 `json:"time_sum"`
	}
	err := c.get(ctx, c.GameplayBase+"/games/"+url.PathEscape(productID)+"/users/"+url.PathEscape(userID)+"/sessions", token, &body)
	if errors.Is(err, errNotFound) {
		return 0, nil
	}
	return body.TimeSum, err
}
