package battlenet

import (
	"context"
	"fmt"
)

// SC2Profile is a member's StarCraft II summary (primary toon).
type SC2Profile struct {
	DisplayName string `json:"display_name"`
	Race        string `json:"race"`
	League      string `json:"league"`
}

// SC2Profile fetches the member's first StarCraft II toon and its 1v1 career
// summary. accountId is the Blizzard account id (OAuth sub). Returns nil if the
// member has no SC2 profile.
func (c *Connector) SC2Profile(ctx context.Context, token, accountId, region string) (*SC2Profile, error) {
	base := c.apiBase(region)

	var toons []struct {
		ProfileID   string `json:"profileId"`
		RegionID    int    `json:"regionId"`
		RealmID     int    `json:"realmId"`
		DisplayName string `json:"displayName"`
	}
	found, err := c.getProfileJSON(ctx, base+"/sc2/player/"+accountId, token, &toons)
	if err != nil || !found || len(toons) == 0 {
		return nil, err
	}
	t := toons[0]

	endpoint := fmt.Sprintf("%s/sc2/profile/%d/%d/%s?locale=en_US", base, t.RegionID, t.RealmID, t.ProfileID)
	var profile struct {
		Summary struct {
			DisplayName string `json:"displayName"`
		} `json:"summary"`
		Career struct {
			PrimaryRace      string `json:"primaryRace"`
			Current1v1League string `json:"current1v1LeagueName"`
		} `json:"career"`
	}
	if found, err := c.getProfileJSON(ctx, endpoint, token, &profile); err != nil || !found {
		return nil, err
	}

	name := profile.Summary.DisplayName
	if name == "" {
		name = t.DisplayName
	}
	return &SC2Profile{
		DisplayName: name,
		Race:        profile.Career.PrimaryRace,
		League:      profile.Career.Current1v1League,
	}, nil
}
