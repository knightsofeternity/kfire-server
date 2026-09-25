package api

import (
	"errors"
	"sort"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

// maxRecapWindow bounds how much of the history one call may read. Without it
// a year-long window would scan the whole match history on every request,
// while a week already covers an evening several times over.
const maxRecapWindow = 7 * 24 * time.Hour

// errRecapRange carries the reason a window was refused, so the caller gets a
// sentence it can show instead of an empty result.
var errRecapRange = errors.New("invalid recap range")

// recapWindow is the half-open interval [From, To) a recap covers.
type recapWindow struct {
	From time.Time
	To   time.Time
}

// parseRecapWindow validates the two query parameters. It is deliberately a
// pure function: this is the one place in the feature where an off-by-one is
// both easy to write and invisible in production.
func parseRecapWindow(from, to string) (recapWindow, string, error) {
	if from == "" || to == "" {
		return recapWindow{}, "from and to are required", errRecapRange
	}
	f, err := time.Parse(time.RFC3339, from)
	if err != nil {
		return recapWindow{}, "from must be RFC 3339", errRecapRange
	}
	t, err := time.Parse(time.RFC3339, to)
	if err != nil {
		return recapWindow{}, "to must be RFC 3339", errRecapRange
	}
	if !t.After(f) {
		return recapWindow{}, "to must be strictly after from", errRecapRange
	}
	if t.Sub(f) > maxRecapWindow {
		return recapWindow{}, "the range cannot exceed 7 days", errRecapRange
	}
	return recapWindow{From: f.UTC(), To: t.UTC()}, "", nil
}

// recapMember is the identity half of a per-member line. It says which member
// a match CAME FROM. Who played with whom is known only through match_key
// (see groupRocketLeague), never guessed from two members playing at the same
// minute.
type recapMember struct {
	UserID    string
	Username  string
	AvatarURL *string
}

// rocketLeagueRecapMember is one member's Rocket League record over a window.
type rocketLeagueRecapMember struct {
	recapMember
	Matches         int
	Wins            int
	Losses          int
	Draws           int
	MVPs            int
	Goals           int
	Assists         int
	Saves           int
	Shots           int
	Demos           int
	Score           int
	PlayTimeSeconds int
}

// hearthstoneRecapMember is one member's Hearthstone record over a window.
// Nothing is shared with the Rocket League record on purpose: one counts
// goals, the other a finishing position.
type hearthstoneRecapMember struct {
	recapMember
	Matches      int
	Wins         int
	Losses       int
	Draws        int
	Ranked       int // matches carrying a placement (Battlegrounds)
	PlacementSum int
	Top4         int
}

// recapGameBlock is one game's part of the summary. Members holds either
// rocketLeagueRecapMember or hearthstoneRecapMember values, already rendered.
type recapGameBlock struct {
	store.RecapGame
	Matches int
	Members []fiber.Map
}

// tallyResult adds one match result to a win/loss/draw triple.
func tallyResult(result string, wins, losses, draws *int) {
	switch result {
	case "win":
		*wins++
	case "loss":
		*losses++
	default:
		*draws++
	}
}

// aggregateRocketLeague folds matches into one block per game, and inside each
// block one line per member.
//
// Grouping is by game id rather than by slug because the same game can exist
// twice in the catalog (two stores, two rows) and a recap must not merge two
// distinct catalog entries behind one identifier.
func aggregateRocketLeague(matches []store.RecapRocketLeagueMatch) []recapGameBlock {
	type key struct{ game, user string }
	byMember := map[key]*rocketLeagueRecapMember{}
	games := map[string]*recapGameBlock{}
	var order []string

	for _, m := range matches {
		g, ok := games[m.GameID]
		if !ok {
			g = &recapGameBlock{RecapGame: m.RecapGame}
			games[m.GameID] = g
			order = append(order, m.GameID)
		}
		g.Matches++

		k := key{m.GameID, m.UserID}
		s, ok := byMember[k]
		if !ok {
			s = &rocketLeagueRecapMember{recapMember: recapMember{
				UserID: m.UserID, Username: m.Username, AvatarURL: m.AvatarURL}}
			byMember[k] = s
		}
		s.Matches++
		tallyResult(m.Result, &s.Wins, &s.Losses, &s.Draws)
		if m.MVP {
			s.MVPs++
		}
		s.Goals += m.Goals
		s.Assists += m.Assists
		s.Saves += m.Saves
		s.Shots += m.Shots
		s.Demos += m.Demos
		s.Score += m.Score
		s.PlayTimeSeconds += m.DurationSeconds
	}

	blocks := make([]recapGameBlock, 0, len(order))
	for _, id := range order {
		g := games[id]
		var members []*rocketLeagueRecapMember
		for k, s := range byMember {
			if k.game == id {
				members = append(members, s)
			}
		}
		sort.SliceStable(members, func(i, j int) bool {
			if members[i].Matches != members[j].Matches {
				return members[i].Matches > members[j].Matches
			}
			return members[i].Username < members[j].Username
		})
		for _, s := range members {
			g.Members = append(g.Members, rocketLeagueMemberJSON(*s))
		}
		blocks = append(blocks, *g)
	}
	sortRecapBlocks(blocks)
	return blocks
}

// aggregateHearthstone is the Hearthstone counterpart of aggregateRocketLeague.
func aggregateHearthstone(matches []store.RecapHearthstoneMatch) []recapGameBlock {
	type key struct{ game, user string }
	byMember := map[key]*hearthstoneRecapMember{}
	games := map[string]*recapGameBlock{}
	var order []string

	for _, m := range matches {
		g, ok := games[m.GameID]
		if !ok {
			g = &recapGameBlock{RecapGame: m.RecapGame}
			games[m.GameID] = g
			order = append(order, m.GameID)
		}
		g.Matches++

		k := key{m.GameID, m.UserID}
		s, ok := byMember[k]
		if !ok {
			s = &hearthstoneRecapMember{recapMember: recapMember{
				UserID: m.UserID, Username: m.Username, AvatarURL: m.AvatarURL}}
			byMember[k] = s
		}
		s.Matches++
		tallyResult(m.Result, &s.Wins, &s.Losses, &s.Draws)
		// A match without a placement says nothing about how it went, so it
		// weighs on neither the average nor the top-4 count.
		if m.Placement != nil {
			s.Ranked++
			s.PlacementSum += *m.Placement
			if *m.Placement <= 4 {
				s.Top4++
			}
		}
	}

	blocks := make([]recapGameBlock, 0, len(order))
	for _, id := range order {
		g := games[id]
		var members []*hearthstoneRecapMember
		for k, s := range byMember {
			if k.game == id {
				members = append(members, s)
			}
		}
		// By match count then name, the order every other per-member aggregate
		// uses. It is NOT a ranking: the page sorts on what it displays.
		sort.SliceStable(members, func(i, j int) bool {
			if members[i].Matches != members[j].Matches {
				return members[i].Matches > members[j].Matches
			}
			return members[i].Username < members[j].Username
		})
		for _, s := range members {
			g.Members = append(g.Members, hearthstoneMemberJSON(*s))
		}
		blocks = append(blocks, *g)
	}
	sortRecapBlocks(blocks)
	return blocks
}

// sortRecapBlocks orders games by match count then name, so the game that
// carried the evening comes first.
func sortRecapBlocks(blocks []recapGameBlock) {
	sort.SliceStable(blocks, func(i, j int) bool {
		if blocks[i].Matches != blocks[j].Matches {
			return blocks[i].Matches > blocks[j].Matches
		}
		return blocks[i].GameName < blocks[j].GameName
	})
}

func recapMemberJSON(m recapMember) fiber.Map {
	out := fiber.Map{"user_id": m.UserID, "username": m.Username}
	if m.AvatarURL != nil {
		out["avatar_url"] = *m.AvatarURL
	}
	return out
}

func rocketLeagueMemberJSON(s rocketLeagueRecapMember) fiber.Map {
	out := recapMemberJSON(s.recapMember)
	out["matches"] = s.Matches
	out["wins"] = s.Wins
	out["losses"] = s.Losses
	out["draws"] = s.Draws
	out["mvps"] = s.MVPs
	out["goals"] = s.Goals
	out["assists"] = s.Assists
	out["saves"] = s.Saves
	out["shots"] = s.Shots
	out["demos"] = s.Demos
	out["score"] = s.Score
	out["play_time_seconds"] = s.PlayTimeSeconds
	return out
}

func hearthstoneMemberJSON(s hearthstoneRecapMember) fiber.Map {
	out := recapMemberJSON(s.recapMember)
	out["matches"] = s.Matches
	out["wins"] = s.Wins
	out["losses"] = s.Losses
	out["draws"] = s.Draws
	out["ranked"] = s.Ranked
	out["top4"] = s.Top4
	// Null rather than zero when nothing carried a placement: an average of
	// nothing is not a first place.
	out["avg_placement"] = nil
	if s.Ranked > 0 {
		out["avg_placement"] = float64(s.PlacementSum) / float64(s.Ranked)
	}
	return out
}

// recapGameIcon adds a link to the image cache, and only when the catalog
// actually holds an icon for that game.
//
// Emitting the link unconditionally would be simpler and wrong: the cache
// answers 404 for a game with no source image, and the page would draw a
// broken image beside a name that is perfectly fine. Absence is the signal,
// so it is decided here rather than guessed by the browser.
func recapGameIcon(out fiber.Map, base string, game store.RecapGame) {
	if game.GameIcon != nil {
		out["icon_url"] = base + "/img/games/" + game.GameID + "/icon"
	}
}

// recapEntryJSON is the shared head of a timeline entry: when, which member it
// came from, and which game.
//
// base is the server's public URL, threaded down rather than read from a
// global so these builders stay plain functions the tests can call.
func recapEntryJSON(base string, owner store.RecapMatchOwner, game store.RecapGame, playedAt time.Time) fiber.Map {
	out := recapMemberJSON(recapMember{owner.UserID, owner.Username, owner.AvatarURL})
	out["played_at"] = playedAt.UTC()
	out["game_id"] = game.GameID
	out["game_slug"] = game.GameSlug
	out["game_name"] = game.GameName
	recapGameIcon(out, base, game)
	return out
}

func rocketLeagueEntryJSON(base string, m store.RecapRocketLeagueMatch) fiber.Map {
	out := recapEntryJSON(base, m.RecapMatchOwner, m.RecapGame, m.PlayedAt)
	out["result"] = m.Result
	out["playlist"] = m.Playlist
	out["team_size"] = m.TeamSize
	out["player_team"] = m.PlayerTeam
	out["team_blue_score"] = m.TeamBlueScore
	out["team_orange_score"] = m.TeamOrangeScore
	out["goals"] = m.Goals
	out["assists"] = m.Assists
	out["saves"] = m.Saves
	out["shots"] = m.Shots
	out["score"] = m.Score
	out["demos"] = m.Demos
	out["mvp"] = m.MVP
	out["duration_seconds"] = m.DurationSeconds
	out["id"] = m.ID
	out["has_scoreboard"] = m.MatchKey != nil
	return out
}

func hearthstoneEntryJSON(base string, m store.RecapHearthstoneMatch) fiber.Map {
	out := recapEntryJSON(base, m.RecapMatchOwner, m.RecapGame, m.PlayedAt)
	out["mode"] = m.Mode
	out["result"] = m.Result
	out["turns"] = m.Turns
	out["placement"] = m.Placement
	out["hero_card_id"] = m.HeroCardID
	return out
}

// rocketLeagueGroup is one Rocket League match as the timeline shows it: every
// member who reported the same match_key, on one line.
//
// The reference, whose report the line and its scoreboard are built from, is
// the first member by name, so the line does not depend on who reported
// first. The time shown is the earliest report of the group.
type rocketLeagueGroup struct {
	ref     store.RecapRocketLeagueMatch
	members []store.RecapMatchOwner
	mixed   bool
	at      time.Time
}

// groupRocketLeague folds the reports that share a match_key. Reports without
// one stay alone, as before the scoreboard existed. rl arrives oldest first,
// so a group's first report is its earliest and the groups stay in order.
func groupRocketLeague(rl []store.RecapRocketLeagueMatch) []rocketLeagueGroup {
	out := make([]rocketLeagueGroup, 0, len(rl))
	byKey := map[string]int{}
	for _, m := range rl {
		if m.MatchKey != nil {
			if i, ok := byKey[*m.MatchKey]; ok {
				g := &out[i]
				g.members = append(g.members, m.RecapMatchOwner)
				if m.PlayerTeam != g.ref.PlayerTeam {
					g.mixed = true
				}
				if m.Username < g.ref.Username {
					g.ref = m
				}
				continue
			}
			byKey[*m.MatchKey] = len(out)
		}
		out = append(out, rocketLeagueGroup{
			ref: m, members: []store.RecapMatchOwner{m.RecapMatchOwner}, at: m.PlayedAt,
		})
	}
	for i := range out {
		ms := out[i].members
		sort.SliceStable(ms, func(a, b int) bool { return ms[a].Username < ms[b].Username })
	}
	return out
}

// rocketLeagueGroupJSON renders a group: the reference's entry, the earliest
// time, every member present, and whether members played on both sides.
func rocketLeagueGroupJSON(base string, g rocketLeagueGroup) fiber.Map {
	out := rocketLeagueEntryJSON(base, g.ref)
	out["played_at"] = g.at.UTC()
	members := make([]fiber.Map, 0, len(g.members))
	for _, o := range g.members {
		members = append(members, recapMemberJSON(recapMember{o.UserID, o.Username, o.AvatarURL}))
	}
	out["members"] = members
	out["mixed"] = g.mixed
	return out
}

// mergeRecapTimeline interleaves the per-game lists into the single
// oldest-first order the evening actually happened in.
//
// Rocket League reports are first folded into matches: the reports that
// carry the same match_key are one game played together, the only link
// between two members that is a fact rather than a guess. On an exact tie the
// Rocket League entry comes first, which is arbitrary but stable.
func mergeRecapTimeline(base string, rl []store.RecapRocketLeagueMatch, hs []store.RecapHearthstoneMatch) []fiber.Map {
	groups := groupRocketLeague(rl)
	out := make([]fiber.Map, 0, len(groups)+len(hs))
	i, j := 0, 0
	for i < len(groups) && j < len(hs) {
		if hs[j].PlayedAt.Before(groups[i].at) {
			out = append(out, hearthstoneEntryJSON(base, hs[j]))
			j++
			continue
		}
		out = append(out, rocketLeagueGroupJSON(base, groups[i]))
		i++
	}
	for ; i < len(groups); i++ {
		out = append(out, rocketLeagueGroupJSON(base, groups[i]))
	}
	for ; j < len(hs); j++ {
		out = append(out, hearthstoneEntryJSON(base, hs[j]))
	}
	return out
}

// GET /api/v1/recap?from=<RFC3339>&to=<RFC3339>  (authenticated)
//
// One call returns the whole evening: the per-game, per-member summary and the
// single all-games timeline. Splitting it per game would force the page to
// make three calls and stitch them back together for nothing.
//
// This is NOT /sessions, which serves presence sessions.
func (h *handlers) recap(c *fiber.Ctx) error {
	w, msg, err := parseRecapWindow(c.Query("from"), c.Query("to"))
	if err != nil {
		// An invalid window is answered with a sentence, not an empty recap:
		// an empty result is the normal answer to a badly chosen evening, and
		// the two must not look alike.
		return errorJSON(c, fiber.StatusBadRequest, "invalid_range", msg)
	}

	// Who is asking decides whose matches they may see: a recap is a listing of
	// recent sessions, so it honours the same privacy toggle they do.
	claims := mustClaims(c)
	viewer := store.RecapViewer{UserID: claims.UserID, IsAdmin: claims.Role == "admin"}

	rl, err := h.store.RocketLeagueMatchesBetween(c.Context(), w.From, w.To, viewer)
	if err != nil {
		return err
	}
	hs, err := h.store.HearthstoneMatchesBetween(c.Context(), w.From, w.To, viewer)
	if err != nil {
		return err
	}

	blocks := append(aggregateRocketLeague(rl), aggregateHearthstone(hs)...)
	sortRecapBlocks(blocks)
	games := make([]fiber.Map, 0, len(blocks))
	for _, b := range blocks {
		block := fiber.Map{
			"game_id":   b.GameID,
			"game_slug": b.GameSlug,
			"game_name": b.GameName,
			"matches":   b.Matches,
			"members":   b.Members,
		}
		recapGameIcon(block, h.cfg.PublicURL, b.RecapGame)
		games = append(games, block)
	}

	return c.JSON(fiber.Map{
		"from":          w.From,
		"to":            w.To,
		"total_matches": len(rl) + len(hs),
		"games":         games,
		"timeline":      mergeRecapTimeline(h.cfg.PublicURL, rl, hs),
	})
}
