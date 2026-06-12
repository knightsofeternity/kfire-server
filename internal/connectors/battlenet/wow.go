package battlenet

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// WowCharacter is one of a member's WoW characters, enriched with item level,
// Mythic+ rating and raid progress.
type WowCharacter struct {
	Name              string
	RealmSlug         string
	RealmName         string
	Class             string
	Race              string
	Faction           string
	Level             int
	ItemLevel         int
	MythicRating      *float64
	RaidSummary       json.RawMessage
	AchievementPoints int
}

func (c *Connector) apiBase(region string) string {
	if c.APIBase != "" {
		return c.APIBase
	}
	return fmt.Sprintf(defaultAPIBaseTmpl, region)
}

// getProfileJSON performs an authenticated GET against the profile API. A 404 is
// reported via found=false (e.g. a character with no Mythic+ profile), not an error.
func (c *Connector) getProfileJSON(ctx context.Context, endpoint, token string, dst any) (found bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("battlenet wow: HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return false, err
	}
	return true, nil
}

// WowAccountCharacters lists the member's characters for one namespace.
func (c *Connector) WowAccountCharacters(ctx context.Context, token, namespace string) ([]WowCharacter, error) {
	region := namespaceRegion(namespace)
	q := url.Values{"namespace": {namespace}, "locale": {"en_US"}}
	endpoint := c.apiBase(region) + "/profile/user/wow?" + q.Encode()

	var payload struct {
		WowAccounts []struct {
			Characters []struct {
				Name  string `json:"name"`
				Realm struct {
					Slug string `json:"slug"`
					Name string `json:"name"`
				} `json:"realm"`
				PlayableClass struct {
					Name string `json:"name"`
				} `json:"playable_class"`
				PlayableRace struct {
					Name string `json:"name"`
				} `json:"playable_race"`
				Faction struct {
					Name string `json:"name"`
				} `json:"faction"`
				Level int `json:"level"`
			} `json:"characters"`
		} `json:"wow_accounts"`
	}
	found, err := c.getProfileJSON(ctx, endpoint, token, &payload)
	if err != nil || !found {
		return nil, err
	}

	var out []WowCharacter
	for _, acc := range payload.WowAccounts {
		for _, ch := range acc.Characters {
			out = append(out, WowCharacter{
				Name:      ch.Name,
				RealmSlug: ch.Realm.Slug,
				RealmName: ch.Realm.Name,
				Class:     ch.PlayableClass.Name,
				Race:      ch.PlayableRace.Name,
				Faction:   ch.Faction.Name,
				Level:     ch.Level,
			})
		}
	}
	return out, nil
}

// EnrichWowCharacter fills ItemLevel, MythicRating and RaidSummary for one
// character. Missing sub-profiles (no M+, no raids) are left nil.
func (c *Connector) EnrichWowCharacter(ctx context.Context, token, namespace string, ch *WowCharacter) error {
	region := namespaceRegion(namespace)
	base := c.apiBase(region)
	charPath := fmt.Sprintf("/profile/wow/character/%s/%s",
		url.PathEscape(ch.RealmSlug), url.PathEscape(strings.ToLower(ch.Name)))
	q := "?" + url.Values{"namespace": {namespace}, "locale": {"en_US"}}.Encode()

	var summary struct {
		EquippedItemLevel int `json:"equipped_item_level"`
		AchievementPoints int `json:"achievement_points"`
	}
	if _, err := c.getProfileJSON(ctx, base+charPath+q, token, &summary); err != nil {
		return err
	}
	ch.ItemLevel = summary.EquippedItemLevel
	ch.AchievementPoints = summary.AchievementPoints

	var mplus struct {
		CurrentMythicRating struct {
			Rating float64 `json:"rating"`
		} `json:"current_mythic_rating"`
	}
	if found, err := c.getProfileJSON(ctx, base+charPath+"/mythic-keystone-profile"+q, token, &mplus); err != nil {
		return err
	} else if found {
		r := mplus.CurrentMythicRating.Rating
		ch.MythicRating = &r
	}

	var raids json.RawMessage
	if found, err := c.getProfileJSON(ctx, base+charPath+"/encounters/raids"+q, token, &raids); err != nil {
		return err
	} else if found {
		ch.RaidSummary = raids
	}
	return nil
}

// namespaceRegion extracts the region suffix from a namespace
// ("profile-eu" -> "eu", "profile-classic1x-eu" -> "eu").
func namespaceRegion(namespace string) string {
	i := strings.LastIndex(namespace, "-")
	if i < 0 {
		return namespace
	}
	return namespace[i+1:]
}
