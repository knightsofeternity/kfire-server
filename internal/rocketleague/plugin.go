package rocketleague

import (
	"context"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

// recentMatches est le nombre de matchs demandé par la fiche membre.
const recentMatches = 10

// Plugin montre le bilan Rocket League de la guilde.
type Plugin struct {
	st *store.Store
}

// New construit le plugin.
func New(st *store.Store) *Plugin { return &Plugin{st: st} }

func (p *Plugin) ID() string      { return "rocket-league" }
func (p *Plugin) Name() string    { return "Rocket League" }
func (p *Plugin) Slugs() []string { return []string{"rocket-league"} }

// Connector rend une chaîne vide : Rocket League n'a pas de couche de
// credentials. Même situation que Hearthstone.
func (p *Plugin) Connector() string { return "" }

// Available est toujours vrai. Tous les autres plugins dépendent d'un connecteur
// détenant une clé ; celui-ci n'a aucune clé à détenir, puisque la donnée arrive
// de la machine du membre. L'interrupteur admin du registre reste le moyen de
// l'éteindre.
func (p *Plugin) Available() bool { return true }

// Refresh ne fait rien : il n'y a rien à crawler. Les résultats arrivent par le
// plan de contrôle quand un membre termine un match.
func (p *Plugin) Refresh(ctx context.Context, userID, gameSlug string) {}

// GameDetail rend le bilan de la guilde, une entrée par membre.
func (p *Plugin) GameDetail(ctx context.Context, _ string, g store.Game) (map[string]any, error) {
	stats, err := p.st.RocketLeagueStatsByGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	cards := make([]map[string]any, len(stats))
	for i, s := range stats {
		m := map[string]any{
			"user_id": s.UserID, "username": s.Username,
			"matches": s.Matches, "wins": s.Wins, "mvps": s.MVPs,
			"goals": s.Goals, "assists": s.Assists, "saves": s.Saves,
			"shots": s.Shots, "demos": s.Demos, "score": s.Score,
			"play_time_seconds": s.PlayTime,
			"last_played_at":    s.LastPlayedAt,
		}
		if s.AvatarURL != nil {
			m["avatar_url"] = *s.AvatarURL
		}
		cards[i] = m
	}
	return map[string]any{"rl_players": cards}, nil
}

// UserGameDetail rend le bloc d'un membre : ses derniers matchs.
//
// L'identifiant de playlist n'est PAS traduit ici. Le navigateur en fait un
// libellé, parce que l'identifiant est le fait et que le libellé est de la
// présentation, différente selon la langue. Même choix que hero_card_id pour
// Hearthstone.
func (p *Plugin) UserGameDetail(ctx context.Context, targetUserID string, g store.Game) (map[string]any, error) {
	matches, err := p.st.RocketLeagueRecentMatches(ctx, targetUserID, g.ID, recentMatches)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, len(matches))
	for i, m := range matches {
		out[i] = map[string]any{
			"playlist": m.Playlist, "team_size": m.TeamSize,
			"player_team": m.PlayerTeam,
			"team_blue_score": m.TeamBlueScore, "team_orange_score": m.TeamOrangeScore,
			"result": m.Result, "goals": m.Goals, "assists": m.Assists,
			"saves": m.Saves, "shots": m.Shots, "score": m.Score, "demos": m.Demos,
			"mvp": m.MVP, "duration_seconds": m.DurationSeconds,
			"played_at": m.PlayedAt,
		}
	}
	return map[string]any{"rl_matches": out}, nil
}
