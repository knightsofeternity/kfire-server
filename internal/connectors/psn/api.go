package psn

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Friends returns the bot's friends' account ids.
func (c *Connector) Friends(ctx context.Context, token string) ([]string, error) {
	var body struct {
		Friends []string `json:"friends"`
	}
	err := c.do(ctx, http.MethodGet, c.APIBase+"/userProfile/v1/internal/users/me/friends?limit=1000", token, &body)
	return body.Friends, err
}

// ReceivedRequests returns the account ids waiting for the bot to accept them.
func (c *Connector) ReceivedRequests(ctx context.Context, token string) ([]string, error) {
	var body struct {
		ReceivedRequests []struct {
			AccountID string `json:"accountId"`
		} `json:"receivedRequests"`
	}
	err := c.do(ctx, http.MethodGet, c.APIBase+"/userProfile/v1/internal/users/me/friends/receivedRequests?limit=100", token, &body)
	out := make([]string, 0, len(body.ReceivedRequests))
	for _, r := range body.ReceivedRequests {
		out = append(out, r.AccountID)
	}
	return out, err
}

// AcceptFriend accepts a pending request from accountID.
func (c *Connector) AcceptFriend(ctx context.Context, token, accountID string) error {
	return c.do(ctx, http.MethodPut, c.APIBase+"/userProfile/v1/internal/users/me/friends/"+url.PathEscape(accountID), token, nil)
}

// RemoveFriend ends the friendship with accountID.
func (c *Connector) RemoveFriend(ctx context.Context, token, accountID string) error {
	return c.do(ctx, http.MethodDelete, c.APIBase+"/userProfile/v1/internal/users/me/friends/"+url.PathEscape(accountID), token, nil)
}

// Presence is what a friend is doing right now.
type Presence struct {
	AccountID string
	Online    bool
	// Playing is empty when the friend is online outside a game.
	TitleID   string
	TitleName string
}

// Presences reads up to 100 friends' presence in one call.
func (c *Connector) Presences(ctx context.Context, token string, accountIDs []string) ([]Presence, error) {
	if len(accountIDs) == 0 {
		return nil, nil
	}
	if len(accountIDs) > 100 {
		return nil, fmt.Errorf("psn: presences: %d ids, at most 100 per call", len(accountIDs))
	}
	var body struct {
		BasicPresences []struct {
			AccountID           string `json:"accountId"`
			PrimaryPlatformInfo struct {
				OnlineStatus string `json:"onlineStatus"`
			} `json:"primaryPlatformInfo"`
			GameTitleInfoList []struct {
				NpTitleID string `json:"npTitleId"`
				TitleName string `json:"titleName"`
			} `json:"gameTitleInfoList"`
		} `json:"basicPresences"`
	}
	u := c.APIBase + "/userProfile/v1/internal/users/basicPresences?type=primary&accountIds=" + strings.Join(accountIDs, ",")
	if err := c.do(ctx, http.MethodGet, u, token, &body); err != nil {
		return nil, err
	}
	out := make([]Presence, 0, len(body.BasicPresences))
	for _, p := range body.BasicPresences {
		pr := Presence{AccountID: p.AccountID, Online: p.PrimaryPlatformInfo.OnlineStatus == "online"}
		if len(p.GameTitleInfoList) > 0 {
			pr.TitleID = p.GameTitleInfoList[0].NpTitleID
			pr.TitleName = p.GameTitleInfoList[0].TitleName
		}
		out = append(out, pr)
	}
	return out, nil
}

// PlayedGame is one PS4/PS5 game a friend has played.
type PlayedGame struct {
	TitleID      string
	Name         string
	ImageURL     string
	ConceptID    string
	ConceptTitle []string // every title id of the same concept (PS4, PS5, regions)
	Seconds      int64
	LastPlayed   time.Time
}

// PlayedGames reads a friend's whole played-games list, page by page.
func (c *Connector) PlayedGames(ctx context.Context, token, accountID string) ([]PlayedGame, error) {
	var out []PlayedGame
	for offset := 0; ; offset += 200 {
		var body struct {
			Titles []struct {
				TitleID            string   `json:"titleId"`
				Name               string   `json:"name"`
				ImageURL           string   `json:"imageUrl"`
				PlayDuration       string   `json:"playDuration"`
				LastPlayedDateTime sonyTime `json:"lastPlayedDateTime"`
				Concept            struct {
					ID       json.Number `json:"id"`
					TitleIDs []string    `json:"titleIds"`
				} `json:"concept"`
			} `json:"titles"`
			TotalItemCount int `json:"totalItemCount"`
		}
		u := fmt.Sprintf("%s/gamelist/v2/users/%s/titles?categories=ps4_game,ps5_native_game&limit=200&offset=%d",
			c.APIBase, url.PathEscape(accountID), offset)
		if err := c.do(ctx, http.MethodGet, u, token, &body); err != nil {
			return nil, err
		}
		for _, t := range body.Titles {
			out = append(out, PlayedGame{
				TitleID: t.TitleID, Name: t.Name, ImageURL: t.ImageURL,
				ConceptID: t.Concept.ID.String(), ConceptTitle: t.Concept.TitleIDs,
				Seconds: ParseDuration(t.PlayDuration), LastPlayed: t.LastPlayedDateTime.Time,
			})
		}
		if len(body.Titles) == 0 || len(out) >= body.TotalItemCount {
			return out, nil
		}
	}
}

// TrophyTitle is one game in a friend's trophy list.
type TrophyTitle struct {
	NpCommunicationID string
	Service           string
	Name              string
	LastUpdated       time.Time
}

// TrophyTitles reads a friend's trophy list, most recently updated first.
func (c *Connector) TrophyTitles(ctx context.Context, token, accountID string) ([]TrophyTitle, error) {
	var body struct {
		TrophyTitles []struct {
			NpCommunicationID   string   `json:"npCommunicationId"`
			NpServiceName       string   `json:"npServiceName"`
			TrophyTitleName     string   `json:"trophyTitleName"`
			LastUpdatedDateTime sonyTime `json:"lastUpdatedDateTime"`
		} `json:"trophyTitles"`
	}
	u := c.APIBase + "/trophy/v1/users/" + url.PathEscape(accountID) + "/trophyTitles?limit=800"
	if err := c.do(ctx, http.MethodGet, u, token, &body); err != nil {
		return nil, err
	}
	out := make([]TrophyTitle, 0, len(body.TrophyTitles))
	for _, t := range body.TrophyTitles {
		out = append(out, TrophyTitle{t.NpCommunicationID, t.NpServiceName, t.TrophyTitleName, t.LastUpdatedDateTime.Time})
	}
	return out, nil
}

// Trophy is one EARNED trophy, with its name and icon.
type Trophy struct {
	ID       int
	Name     string
	IconURL  string
	Type     string
	EarnedAt time.Time
}

// EarnedTrophies returns a friend's earned trophies for one game, named in
// French (the instance's language, like the rest of the guild's pages).
func (c *Connector) EarnedTrophies(ctx context.Context, token, accountID string, t TrophyTitle) ([]Trophy, error) {
	var earned struct {
		Trophies []struct {
			TrophyID       int      `json:"trophyId"`
			Earned         bool     `json:"earned"`
			EarnedDateTime sonyTime `json:"earnedDateTime"`
		} `json:"trophies"`
	}
	path := "/npCommunicationIds/" + url.PathEscape(t.NpCommunicationID) + "/trophyGroups/all/trophies?npServiceName=" + url.QueryEscape(t.Service)
	if err := c.do(ctx, http.MethodGet, c.APIBase+"/trophy/v1/users/"+url.PathEscape(accountID)+path, token, &earned); err != nil {
		return nil, err
	}
	var meta struct {
		Trophies []struct {
			TrophyID      int    `json:"trophyId"`
			TrophyName    string `json:"trophyName"`
			TrophyIconURL string `json:"trophyIconUrl"`
			TrophyType    string `json:"trophyType"`
		} `json:"trophies"`
	}
	if err := c.do(ctx, http.MethodGet, c.APIBase+"/trophy/v1"+path, token, &meta); err != nil {
		return nil, err
	}
	byID := make(map[int]int, len(meta.Trophies))
	for i, m := range meta.Trophies {
		byID[m.TrophyID] = i
	}
	var out []Trophy
	for _, e := range earned.Trophies {
		if !e.Earned {
			continue
		}
		tr := Trophy{ID: e.TrophyID, EarnedAt: e.EarnedDateTime.Time}
		if i, ok := byID[e.TrophyID]; ok {
			m := meta.Trophies[i]
			tr.Name, tr.IconURL, tr.Type = m.TrophyName, m.TrophyIconURL, m.TrophyType
		}
		out = append(out, tr)
	}
	return out, nil
}

// sonyTime reads Sony's timestamps leniently: an empty or odd value becomes
// the zero time instead of failing the decode of a whole page.
type sonyTime struct{ time.Time }

func (t *sonyTime) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) != nil || s == "" {
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339Nano, s); err == nil {
		t.Time = parsed
	}
	return nil
}
