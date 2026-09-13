package riot

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// rankedQueues are the only League queues KFIRE displays.
var rankedQueues = map[string]bool{
	"RANKED_SOLO_5x5": true,
	"RANKED_FLEX_SR":  true,
}

// RankEntry is one ranked queue standing.
type RankEntry struct {
	Queue     string `json:"queue"`
	Tier      string `json:"tier"`
	Division  string `json:"division"`
	LP        int    `json:"lp"`
	Wins      int    `json:"wins"`
	Losses    int    `json:"losses"`
	HotStreak bool   `json:"hot_streak"`
}

// ChampionMastery is one champion's mastery standing. ImageID is Riot's
// internal name for the champion, used to build asset URLs; Name is the
// display name and the two differ for several champions.
type ChampionMastery struct {
	ChampionID int    `json:"champion_id"`
	Name       string `json:"name"`
	ImageID    string `json:"image_id"`
	IconURL    string `json:"icon_url"`
	Level      int    `json:"level"`
	Points     int    `json:"points"`
}

// MatchResult is one played match, reduced to what the card shows.
type MatchResult struct {
	MatchID         string    `json:"match_id"`
	Win             bool      `json:"win"`
	Champion        string    `json:"champion"`
	Kills           int       `json:"kills"`
	Deaths          int       `json:"deaths"`
	Assists         int       `json:"assists"`
	QueueID         int       `json:"queue_id"`
	DurationSeconds int       `json:"duration_seconds"`
	PlayedAt        time.Time `json:"played_at"`
}

// LiveGame is a match in progress.
// LiveGame is a match in progress. ChampionName and ChampionIcon are filled by
// the caller from Data Dragon, which the connector does not reach: Spectator
// answers with a numeric champion id alone.
type LiveGame struct {
	ChampionID   int       `json:"champion_id"`
	ChampionName string    `json:"champion_name,omitempty"`
	ChampionIcon string    `json:"champion_icon,omitempty"`
	QueueID      int       `json:"queue_id"`
	Mode         string    `json:"mode"`
	StartedAt    time.Time `json:"started_at"`
}

// LeagueEntries returns the member's solo and flex standings. An unranked
// account legitimately returns an empty slice.
func (c *Connector) LeagueEntries(ctx context.Context, platform, puuid string) ([]RankEntry, error) {
	var raw []struct {
		QueueType    string `json:"queueType"`
		Tier         string `json:"tier"`
		Rank         string `json:"rank"`
		LeaguePoints int    `json:"leaguePoints"`
		Wins         int    `json:"wins"`
		Losses       int    `json:"losses"`
		HotStreak    bool   `json:"hotStreak"`
	}
	if err := c.get(ctx, platform, "/lol/league/v4/entries/by-puuid/"+url.PathEscape(puuid), &raw); err != nil {
		return nil, err
	}
	out := make([]RankEntry, 0, len(raw))
	for _, e := range raw {
		if !rankedQueues[e.QueueType] {
			continue
		}
		out = append(out, RankEntry{
			Queue: e.QueueType, Tier: e.Tier, Division: e.Rank, LP: e.LeaguePoints,
			Wins: e.Wins, Losses: e.Losses, HotStreak: e.HotStreak,
		})
	}
	return out, nil
}

// TopChampions returns the member's n highest-mastery champions. Names and
// icons are left empty here; the syncer fills them from Data Dragon.
func (c *Connector) TopChampions(ctx context.Context, platform, puuid string, n int) ([]ChampionMastery, error) {
	var raw []struct {
		ChampionID     int `json:"championId"`
		ChampionLevel  int `json:"championLevel"`
		ChampionPoints int `json:"championPoints"`
	}
	path := "/lol/champion-mastery/v4/champion-masteries/by-puuid/" + url.PathEscape(puuid) +
		"/top?count=" + strconv.Itoa(n)
	if err := c.get(ctx, platform, path, &raw); err != nil {
		return nil, err
	}
	out := make([]ChampionMastery, len(raw))
	for i, m := range raw {
		out[i] = ChampionMastery{
			ChampionID: m.ChampionID, Level: m.ChampionLevel, Points: m.ChampionPoints,
		}
	}
	return out, nil
}

// RecentMatches returns the member's n most recent matches, newest first.
//
// The ids route returns identifiers only, so one detail call per match is
// needed to learn the outcome. The details are fetched in parallel, each
// goroutine writing its own index so Riot's newest-first order survives. A
// detail that fails is skipped rather than failing the whole refresh, so a
// partial answer still shows some recent form.
func (c *Connector) RecentMatches(ctx context.Context, cluster, puuid string, n int) ([]MatchResult, error) {
	var ids []string
	path := fmt.Sprintf("/lol/match/v5/matches/by-puuid/%s/ids?start=0&count=%d", url.PathEscape(puuid), n)
	if err := c.get(ctx, cluster, path, &ids); err != nil {
		return nil, err
	}

	out := make([]MatchResult, len(ids))
	ok := make([]bool, len(ids))
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			m, err := c.matchDetail(ctx, cluster, id, puuid)
			if err != nil {
				return
			}
			out[i], ok[i] = m, true
		}(i, id)
	}
	wg.Wait()

	res := make([]MatchResult, 0, len(ids))
	for i := range ids {
		if ok[i] {
			res = append(res, out[i])
		}
	}
	return res, nil
}

// matchDetail reads one match and keeps only the given member's participation.
func (c *Connector) matchDetail(ctx context.Context, cluster, matchID, puuid string) (MatchResult, error) {
	var raw struct {
		Info struct {
			QueueID          int   `json:"queueId"`
			GameDuration     int   `json:"gameDuration"`
			GameEndTimestamp int64 `json:"gameEndTimestamp"`
			Participants     []struct {
				PUUID        string `json:"puuid"`
				ChampionName string `json:"championName"`
				Win          bool   `json:"win"`
				Kills        int    `json:"kills"`
				Deaths       int    `json:"deaths"`
				Assists      int    `json:"assists"`
			} `json:"participants"`
		} `json:"info"`
	}
	if err := c.get(ctx, cluster, "/lol/match/v5/matches/"+url.PathEscape(matchID), &raw); err != nil {
		return MatchResult{}, err
	}
	for _, p := range raw.Info.Participants {
		if p.PUUID != puuid {
			continue
		}
		return MatchResult{
			MatchID: matchID, Win: p.Win, Champion: p.ChampionName,
			Kills: p.Kills, Deaths: p.Deaths, Assists: p.Assists,
			QueueID:         raw.Info.QueueID,
			DurationSeconds: raw.Info.GameDuration,
			PlayedAt:        time.UnixMilli(raw.Info.GameEndTimestamp).UTC(),
		}, nil
	}
	return MatchResult{}, fmt.Errorf("riot: match %s has no participant %s", matchID, puuid)
}

// ActiveGame returns the member's match in progress, or nil when they are not
// in an observable game. A 404 is the normal answer outside a game and is not
// an error.
func (c *Connector) ActiveGame(ctx context.Context, platform, puuid string) (*LiveGame, error) {
	var raw struct {
		GameQueueConfigID int    `json:"gameQueueConfigId"`
		GameStartTime     int64  `json:"gameStartTime"`
		GameMode          string `json:"gameMode"`
		Participants      []struct {
			PUUID      string `json:"puuid"`
			ChampionID int    `json:"championId"`
		} `json:"participants"`
	}
	path := "/lol/spectator/v5/active-games/by-summoner/" + url.PathEscape(puuid)
	if err := c.get(ctx, platform, path, &raw); err != nil {
		if NotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	for _, p := range raw.Participants {
		if p.PUUID != puuid {
			continue
		}
		return &LiveGame{
			ChampionID: p.ChampionID,
			QueueID:    raw.GameQueueConfigID,
			Mode:       raw.GameMode,
			StartedAt:  time.UnixMilli(raw.GameStartTime).UTC(),
		}, nil
	}
	return nil, nil
}
