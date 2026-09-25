package rocketleague

import (
	"sort"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

// ScoreRow is one line of a team. A named member carries UserID and Username;
// anyone else carries AnonIndex instead (1, 2, ... within the team), and the
// page turns it into "Teammate 1" or "Opponent 2".
type ScoreRow struct {
	UserID    string
	Username  string
	AvatarURL *string
	AnonIndex int
	Player    store.RocketLeaguePlayer
	MVP       bool
}

// ScoreTeam is one side: 0 is blue, 1 is orange.
type ScoreTeam struct {
	Team   int
	Score  int
	Winner bool
	Rows   []ScoreRow
}

// Scoreboard is a whole match. ReferenceTeam is the side of the member whose
// report it was built from, which decides who is a teammate.
type Scoreboard struct {
	ReferenceTeam int
	Teams         [2]ScoreTeam
}

// BuildScoreboard rebuilds a match from one member's report and the reports
// other members made of the same match.
//
// The reference report brings the whole line-up, anonymous. Every other
// member's report then claims the anonymous line that is theirs: same team and
// same statistics, or failing that the closest score on the same team, since
// two clients can stop reading the stream a frame apart. A member the viewer
// may not see still claims their line, so nobody else takes it, but it stays
// anonymous.
//
// peers must be in a stable order (the store sorts them by name): with two
// equally close candidates, the first peer wins.
func BuildScoreboard(ref store.RocketLeagueReport, others []store.RocketLeaguePlayer, peers []store.RocketLeagueReport) Scoreboard {
	type slot struct {
		row     ScoreRow
		claimed bool
	}
	slots := make([]slot, 0, len(others)+1)
	slots = append(slots, slot{
		row:     ScoreRow{UserID: ref.UserID, Username: ref.Username, AvatarURL: ref.AvatarURL, Player: ref.Line},
		claimed: true,
	})
	for _, p := range others {
		slots = append(slots, slot{row: ScoreRow{Player: p}})
	}

	for _, peer := range peers {
		best, bestDiff := -1, 0
		for i, s := range slots {
			if s.claimed || s.row.Player.Team != peer.Line.Team {
				continue
			}
			if sameLine(s.row.Player, peer.Line) {
				best = i
				break
			}
			d := abs(s.row.Player.Score - peer.Line.Score)
			if best < 0 || d < bestDiff {
				best, bestDiff = i, d
			}
		}
		if best < 0 {
			continue
		}
		slots[best].claimed = true
		if peer.Visible {
			slots[best].row.UserID = peer.UserID
			slots[best].row.Username = peer.Username
			slots[best].row.AvatarURL = peer.AvatarURL
		}
	}

	sb := Scoreboard{ReferenceTeam: ref.Line.Team}
	for t := 0; t < 2; t++ {
		team := ScoreTeam{Team: t, Score: ref.TeamBlueScore}
		if t == 1 {
			team.Score = ref.TeamOrangeScore
		}
		for _, s := range slots {
			if s.row.Player.Team == t {
				team.Rows = append(team.Rows, s.row)
			}
		}
		sort.SliceStable(team.Rows, func(a, b int) bool {
			return team.Rows[a].Player.Score > team.Rows[b].Player.Score
		})
		n := 0
		for i := range team.Rows {
			if team.Rows[i].UserID == "" {
				n++
				team.Rows[i].AnonIndex = n
			}
		}
		sb.Teams[t] = team
	}

	switch {
	case ref.TeamBlueScore > ref.TeamOrangeScore:
		sb.Teams[0].Winner = true
	case ref.TeamOrangeScore > ref.TeamBlueScore:
		sb.Teams[1].Winner = true
	}
	// The MVP is the best score of the winning team, as the game awards it.
	for t := range sb.Teams {
		if sb.Teams[t].Winner && len(sb.Teams[t].Rows) > 0 {
			sb.Teams[t].Rows[0].MVP = true
		}
	}
	return sb
}

func sameLine(a, b store.RocketLeaguePlayer) bool {
	return a.Team == b.Team && a.Score == b.Score && a.Goals == b.Goals &&
		a.Assists == b.Assists && a.Saves == b.Saves && a.Shots == b.Shots
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
