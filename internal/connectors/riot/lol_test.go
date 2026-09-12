package riot

import (
	"context"
	"testing"
)

func TestLeagueEntriesKeepsOnlyRankedQueues(t *testing.T) {
	c := fakeRiot(t, map[string]string{
		"/euw1/lol/league/v4/entries/by-puuid/P1": `[
			{"queueType":"RANKED_SOLO_5x5","tier":"GOLD","rank":"II","leaguePoints":47,
			 "wins":120,"losses":108,"hotStreak":false},
			{"queueType":"RANKED_TFT_DOUBLE_UP","tier":"DIAMOND","rank":"I","leaguePoints":10,
			 "wins":3,"losses":1,"hotStreak":true}
		]`,
	})
	got, err := c.LeagueEntries(context.Background(), "euw1", "P1")
	if err != nil {
		t.Fatalf("LeagueEntries: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("LeagueEntries returned %d entries, want 1 (TFT must be dropped)", len(got))
	}
	e := got[0]
	if e.Queue != "RANKED_SOLO_5x5" || e.Tier != "GOLD" || e.Division != "II" || e.LP != 47 {
		t.Errorf("entry = %+v", e)
	}
}

func TestLeagueEntriesOnAnUnrankedAccountReturnsEmpty(t *testing.T) {
	c := fakeRiot(t, map[string]string{
		"/euw1/lol/league/v4/entries/by-puuid/P1": `[]`,
	})
	got, err := c.LeagueEntries(context.Background(), "euw1", "P1")
	if err != nil {
		t.Fatalf("LeagueEntries: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want no entries, got %d", len(got))
	}
}

func TestTopChampions(t *testing.T) {
	c := fakeRiot(t, map[string]string{
		"/euw1/lol/champion-mastery/v4/champion-masteries/by-puuid/P1/top": `[
			{"championId":202,"championLevel":4,"championPoints":12600},
			{"championId":17,"championLevel":3,"championPoints":11197}
		]`,
	})
	got, err := c.TopChampions(context.Background(), "euw1", "P1", 3)
	if err != nil {
		t.Fatalf("TopChampions: %v", err)
	}
	if len(got) != 2 || got[0].ChampionID != 202 || got[0].Points != 12600 {
		t.Fatalf("got %+v", got)
	}
}

func TestRecentMatchesReadsWinAndChampionFromTheDetail(t *testing.T) {
	c := fakeRiot(t, map[string]string{
		"/europe/lol/match/v5/matches/by-puuid/P1/ids": `["EUW1_1"]`,
		"/europe/lol/match/v5/matches/EUW1_1": `{"info":{"queueId":450,"gameDuration":1528,
			"gameEndTimestamp":1757620440000,
			"participants":[
				{"puuid":"OTHER","championName":"Lux","win":true,"kills":1,"deaths":2,"assists":3},
				{"puuid":"P1","championName":"Mordekaiser","win":false,"kills":11,"deaths":13,"assists":26}
			]}}`,
	})
	got, err := c.RecentMatches(context.Background(), "europe", "P1", 5)
	if err != nil {
		t.Fatalf("RecentMatches: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 match, got %d", len(got))
	}
	m := got[0]
	if m.Champion != "Mordekaiser" || m.Win || m.Kills != 11 || m.QueueID != 450 {
		t.Errorf("match = %+v (must be the P1 participant, not the first one)", m)
	}
	if m.DurationSeconds != 1528 {
		t.Errorf("duration = %d, want 1528", m.DurationSeconds)
	}
}

func TestRecentMatchesSkipsAMatchThatFailsToLoad(t *testing.T) {
	c := fakeRiot(t, map[string]string{
		"/europe/lol/match/v5/matches/by-puuid/P1/ids": `["EUW1_1","EUW1_missing"]`,
		"/europe/lol/match/v5/matches/EUW1_1": `{"info":{"queueId":420,"gameDuration":900,
			"gameEndTimestamp":1757620440000,
			"participants":[{"puuid":"P1","championName":"Jhin","win":true,"kills":5,"deaths":1,"assists":7}]}}`,
	})
	got, err := c.RecentMatches(context.Background(), "europe", "P1", 5)
	if err != nil {
		t.Fatalf("RecentMatches: %v", err)
	}
	if len(got) != 1 || got[0].Champion != "Jhin" {
		t.Fatalf("a failing detail must be skipped, not fatal; got %+v", got)
	}
}

func TestRecentMatchesKeepsNewestFirst(t *testing.T) {
	c := fakeRiot(t, map[string]string{
		"/europe/lol/match/v5/matches/by-puuid/P1/ids": `["EUW1_1","EUW1_2","EUW1_3"]`,
		"/europe/lol/match/v5/matches/EUW1_1": `{"info":{"queueId":420,"gameDuration":100,
			"gameEndTimestamp":3000,"participants":[{"puuid":"P1","championName":"A","win":true}]}}`,
		"/europe/lol/match/v5/matches/EUW1_2": `{"info":{"queueId":420,"gameDuration":100,
			"gameEndTimestamp":2000,"participants":[{"puuid":"P1","championName":"B","win":true}]}}`,
		"/europe/lol/match/v5/matches/EUW1_3": `{"info":{"queueId":420,"gameDuration":100,
			"gameEndTimestamp":1000,"participants":[{"puuid":"P1","championName":"C","win":true}]}}`,
	})
	got, err := c.RecentMatches(context.Background(), "europe", "P1", 5)
	if err != nil {
		t.Fatalf("RecentMatches: %v", err)
	}
	want := []string{"A", "B", "C"}
	if len(got) != 3 {
		t.Fatalf("want 3 matches, got %d", len(got))
	}
	for i, w := range want {
		if got[i].Champion != w {
			t.Errorf("position %d = %q, want %q; Riot's id order must be preserved "+
				"despite the parallel fetch", i, got[i].Champion, w)
		}
	}
}

func TestActiveGameReturnsNilOn404(t *testing.T) {
	c := fakeRiot(t, map[string]string{})
	got, err := c.ActiveGame(context.Background(), "euw1", "P1")
	if err != nil {
		t.Fatalf("a 404 means 'not in game' and must not be an error, got %v", err)
	}
	if got != nil {
		t.Fatalf("want nil, got %+v", got)
	}
}

func TestActiveGameReadsTheMembersChampion(t *testing.T) {
	c := fakeRiot(t, map[string]string{
		"/euw1/lol/spectator/v5/active-games/by-summoner/P1": `{"gameQueueConfigId":450,
			"gameStartTime":1757620000000,"gameMode":"ARAM",
			"participants":[{"puuid":"OTHER","championId":1},{"puuid":"P1","championId":202}]}`,
	})
	got, err := c.ActiveGame(context.Background(), "euw1", "P1")
	if err != nil {
		t.Fatalf("ActiveGame: %v", err)
	}
	if got == nil || got.ChampionID != 202 || got.Mode != "ARAM" {
		t.Fatalf("got %+v", got)
	}
}
