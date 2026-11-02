package gog

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
	"time"
)

// maxAchievementPages bounds pagination of one game's achievements.
const maxAchievementPages = 10

// GameClient returns the Galaxy client a game was built with, which its
// achievements live under. GOG publishes it in the game's public build
// manifest (Windows first, else macOS); ok is false for a game without a
// Galaxy build, which has no achievements to read.
func (c *Connector) GameClient(ctx context.Context, productID string) (id, secret string, ok bool, err error) {
	for _, os := range []string{"windows", "osx"} {
		var builds struct {
			Items []struct {
				Generation int    `json:"generation"`
				Link       string `json:"link"`
			} `json:"items"`
		}
		u := c.ContentBase + "/products/" + url.PathEscape(productID) + "/os/" + os + "/builds?generation=2"
		if err := c.get(ctx, u, "", &builds); err != nil {
			if errors.Is(err, errNotFound) {
				continue
			}
			return "", "", false, err
		}
		for _, b := range builds.Items {
			if b.Generation != 2 || b.Link == "" {
				continue
			}
			raw, err := c.getRaw(ctx, b.Link, "")
			if err != nil {
				return "", "", false, err
			}
			var m struct {
				ClientID     string `json:"clientId"`
				ClientSecret string `json:"clientSecret"`
			}
			if err := json.Unmarshal(inflate(raw), &m); err != nil {
				return "", "", false, err
			}
			if m.ClientID != "" && m.ClientSecret != "" {
				return m.ClientID, m.ClientSecret, true, nil
			}
			break // the newest build has no client: older ones will not either
		}
	}
	return "", "", false, nil
}

// inflate undoes the zlib compression of a build manifest, and leaves a plain
// JSON body as it is.
func inflate(raw []byte) []byte {
	r, err := zlib.NewReader(bytes.NewReader(raw))
	if err != nil {
		return raw
	}
	defer r.Close()
	out, err := io.ReadAll(io.LimitReader(r, maxBody))
	if err != nil {
		return raw
	}
	return out
}

// Achievement is one achievement the member unlocked.
type Achievement struct {
	ID         string
	Name       string
	Icon       string
	UnlockedAt time.Time
}

// Achievements returns the achievements the member unlocked in one game,
// read with that game's own token (GameToken).
func (c *Connector) Achievements(ctx context.Context, gameToken, gameClientID, userID string) ([]Achievement, error) {
	var out []Achievement
	seen, pageToken := 0, ""
	for page := 0; page < maxAchievementPages; page++ {
		u := c.GameplayBase + "/clients/" + url.PathEscape(gameClientID) + "/users/" + url.PathEscape(userID) + "/achievements"
		if pageToken != "" {
			u += "?page_token=" + url.QueryEscape(pageToken)
		}
		var body struct {
			TotalCount int    `json:"total_count"`
			PageToken  string `json:"page_token"`
			Items      []struct {
				AchievementID string  `json:"achievement_id"`
				Name          string  `json:"name"`
				ImageUnlocked string  `json:"image_url_unlocked"`
				DateUnlocked  *string `json:"date_unlocked"`
			} `json:"items"`
		}
		err := c.get(ctx, u, gameToken, &body)
		if errors.Is(err, errNotFound) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		for _, it := range body.Items {
			if it.DateUnlocked == nil || it.AchievementID == "" {
				continue
			}
			at, ok := parseTime(*it.DateUnlocked)
			if !ok {
				continue
			}
			out = append(out, Achievement{ID: it.AchievementID, Name: it.Name, Icon: httpsURL(it.ImageUnlocked), UnlockedAt: at})
		}
		seen += len(body.Items)
		if len(body.Items) == 0 || seen >= body.TotalCount || body.PageToken == "" || body.PageToken == pageToken {
			return out, nil
		}
		pageToken = body.PageToken
	}
	return out, nil
}

// parseTime reads GOG's dates, written with a "+0000" offset.
func parseTime(s string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05-0700", "2006-01-02T15:04:05.999999-0700"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func httpsURL(u string) string {
	if strings.HasPrefix(u, "//") {
		return "https:" + u
	}
	return u
}
