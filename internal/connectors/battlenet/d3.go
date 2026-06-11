package battlenet

import (
	"context"
	"net/url"
	"strings"
)

// D3Hero is one Diablo III hero on a member's career.
type D3Hero struct {
	Name  string `json:"name"`
	Class string `json:"class"`
	Level int    `json:"level"`
}

// D3Profile is a member's Diablo III career summary.
type D3Profile struct {
	Paragon int      `json:"paragon"`
	Heroes  []D3Hero `json:"heroes"`
}

// D3Profile fetches the member's Diablo III career by BattleTag. The community
// endpoint dashes the BattleTag ("Player#1234" -> "Player-1234").
func (c *Connector) D3Profile(ctx context.Context, token, battleTag, region string) (*D3Profile, error) {
	dashed := strings.ReplaceAll(battleTag, "#", "-")
	endpoint := c.apiBase(region) + "/d3/profile/" + url.PathEscape(dashed) + "/?locale=en_US"

	var payload struct {
		ParagonLevel int      `json:"paragonLevel"`
		Heroes       []D3Hero `json:"heroes"`
	}
	found, err := c.getProfileJSON(ctx, endpoint, token, &payload)
	if err != nil || !found {
		return nil, err
	}
	return &D3Profile{Paragon: payload.ParagonLevel, Heroes: payload.Heroes}, nil
}
