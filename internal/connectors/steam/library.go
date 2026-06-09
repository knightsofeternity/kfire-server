package steam

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// OwnedGame is a game in a member's Steam library.
type OwnedGame struct {
	AppID           string
	Name            string
	PlaytimeForever time.Duration // total lifetime playtime
	IconURL         string
}

// GetOwnedGames returns the member's owned games with lifetime playtime.
// Returns an empty slice (not an error) when the profile hides its game list.
func (c *Connector) GetOwnedGames(ctx context.Context, steamID string) ([]OwnedGame, error) {
	q := url.Values{
		"key":                       {c.APIKey},
		"steamid":                   {steamID},
		"include_appinfo":           {"1"},
		"include_played_free_games": {"1"},
	}
	endpoint := fmt.Sprintf("%s/IPlayerService/GetOwnedGames/v1/?%s", c.APIBase, q.Encode())

	var payload struct {
		Response struct {
			Games []struct {
				AppID           int    `json:"appid"`
				Name            string `json:"name"`
				PlaytimeForever int    `json:"playtime_forever"` // minutes
				ImgIconURL      string `json:"img_icon_url"`
			} `json:"games"`
		} `json:"response"`
	}
	if err := c.getJSON(ctx, endpoint, &payload); err != nil {
		return nil, err
	}

	out := make([]OwnedGame, 0, len(payload.Response.Games))
	for _, g := range payload.Response.Games {
		appID := fmt.Sprintf("%d", g.AppID)
		icon := ""
		if g.ImgIconURL != "" {
			icon = fmt.Sprintf(
				"https://media.steampowered.com/steamcommunity/public/images/apps/%d/%s.jpg",
				g.AppID, g.ImgIconURL)
		}
		out = append(out, OwnedGame{
			AppID:           appID,
			Name:            g.Name,
			PlaytimeForever: time.Duration(g.PlaytimeForever) * time.Minute,
			IconURL:         icon,
		})
	}
	return out, nil
}

// UnlockedAchievement is one achievement a member has earned in a game.
type UnlockedAchievement struct {
	APIName    string
	UnlockedAt time.Time
}

// GetPlayerAchievements returns the achievements the member has unlocked in a
// game. Games without achievements (or a private profile) yield an empty slice
// rather than an error.
func (c *Connector) GetPlayerAchievements(ctx context.Context, steamID, appID string) ([]UnlockedAchievement, error) {
	q := url.Values{"key": {c.APIKey}, "steamid": {steamID}, "appid": {appID}}
	endpoint := fmt.Sprintf("%s/ISteamUserStats/GetPlayerAchievements/v1/?%s", c.APIBase, q.Encode())

	var payload struct {
		PlayerStats struct {
			Success      bool `json:"success"`
			Achievements []struct {
				APIName    string `json:"apiname"`
				Achieved   int    `json:"achieved"`
				UnlockTime int64  `json:"unlocktime"`
			} `json:"achievements"`
		} `json:"playerstats"`
	}
	if err := c.getJSON(ctx, endpoint, &payload); err != nil {
		// 403/400 here usually means "no achievements for this app" - treat as empty.
		return nil, nil
	}

	var out []UnlockedAchievement
	for _, a := range payload.PlayerStats.Achievements {
		if a.Achieved == 1 {
			out = append(out, UnlockedAchievement{
				APIName:    a.APIName,
				UnlockedAt: time.Unix(a.UnlockTime, 0).UTC(),
			})
		}
	}
	return out, nil
}

// AchievementSchema maps an achievement api name to its display name and icon.
type AchievementSchema struct {
	DisplayName string
	IconURL     string
}

// GetGameSchema returns the achievement display metadata for a game.
func (c *Connector) GetGameSchema(ctx context.Context, appID string) (map[string]AchievementSchema, error) {
	q := url.Values{"key": {c.APIKey}, "appid": {appID}}
	endpoint := fmt.Sprintf("%s/ISteamUserStats/GetSchemaForGame/v2/?%s", c.APIBase, q.Encode())

	var payload struct {
		Game struct {
			AvailableGameStats struct {
				Achievements []struct {
					Name        string `json:"name"`
					DisplayName string `json:"displayName"`
					Icon        string `json:"icon"`
				} `json:"achievements"`
			} `json:"availableGameStats"`
		} `json:"game"`
	}
	if err := c.getJSON(ctx, endpoint, &payload); err != nil {
		return nil, nil // schema is best-effort; fall back to api names
	}

	out := make(map[string]AchievementSchema)
	for _, a := range payload.Game.AvailableGameStats.Achievements {
		out[a.Name] = AchievementSchema{DisplayName: a.DisplayName, IconURL: a.Icon}
	}
	return out, nil
}

// getJSON performs a GET and decodes a JSON body, erroring on non-200.
func (c *Connector) getJSON(ctx context.Context, endpoint string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("steam: HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}
