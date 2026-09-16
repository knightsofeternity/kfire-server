// Package rocketleague expose Rocket League comme plugin de jeu. Comme
// Hearthstone, il ne crawle rien : les résultats de match sont rapportés par le
// client de bureau, qui lit la socket de statistiques que le jeu ouvre en local.
package rocketleague

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/matchrecord"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

// clockSkewTolerance est l'avance maximale tolérée sur l'horloge du serveur
// avant qu'un résultat de match soit refusé. Même valeur et même raison que pour
// Hearthstone.
const clockSkewTolerance = 5 * time.Minute

// maxDuration borne la durée d'un match. Une prolongation peut s'éterniser, une
// horloge folle ne doit pas empoisonner le temps de jeu de la guilde.
//
// DOIT rester égal au plafond de duration_seconds dans la migration 0035. Si les
// deux divergent, une charge utile passe la validation puis se fait refuser par
// la contrainte, le hub la classe en erreur transitoire, et le client la
// réémet indéfiniment.
const maxDuration = 7200

// trainingPlaylists sont les identifiants Psyonix que le client ne doit jamais
// rapporter : partie libre, ateliers et entraînement. Ils n'ont pas d'adversaire
// et fausseraient tout ratio.
var trainingPlaylists = map[int]struct{}{0: {}, 9: {}, 19: {}, 21: {}, 73: {}}

// payload est un match terminé, déjà résumé par le client de bureau.
//
// Le flux du jeu porte le nom de TOUS les joueurs du match. Le client s'en sert
// pour calculer mvp et team_size, puis n'envoie que ceci : des faits sur le
// membre, plus deux scores d'équipe qui ne nomment personne.
//
// playlist est l'identifiant numérique brut de Psyonix. Il n'est jamais traduit
// ici : l'identifiant est le fait, le libellé est de la présentation, et il est
// localisé.
type payload struct {
	Playlist        int       `json:"playlist"`
	TeamSize        int       `json:"team_size"`
	PlayerTeam      int       `json:"player_team"`
	TeamBlueScore   int       `json:"team_blue_score"`
	TeamOrangeScore int       `json:"team_orange_score"`
	Result          string    `json:"result"`
	Goals           int       `json:"goals"`
	Assists         int       `json:"assists"`
	Saves           int       `json:"saves"`
	Shots           int       `json:"shots"`
	Score           int       `json:"score"`
	Demos           int       `json:"demos"`
	MVP             bool      `json:"mvp"`
	DurationSeconds int       `json:"duration_seconds"`
	PlayedAt        time.Time `json:"played_at"`
}

// expectedResult rend le résultat que les scores imposent, du point de vue de
// l'équipe du membre.
func expectedResult(playerTeam, blue, orange int) string {
	mine, theirs := blue, orange
	if playerTeam == 1 {
		mine, theirs = orange, blue
	}
	switch {
	case mine > theirs:
		return "win"
	case mine < theirs:
		return "loss"
	default:
		return "draw"
	}
}

// valid dit si la charge utile mérite d'être écrite. La base applique les mêmes
// bornes, mais refuser ici donne au client une erreur claire plutôt qu'un échec
// d'écriture opaque, et permet deux contrôles que le SQL ne sait pas faire : la
// cohérence entre le résultat annoncé et les scores, et le refus d'un MVP sans
// victoire.
func (p payload) valid() bool {
	if p.PlayedAt.IsZero() {
		return false
	}
	if p.Playlist < 0 {
		return false
	}
	if _, training := trainingPlaylists[p.Playlist]; training {
		return false
	}
	if p.TeamSize < 1 || p.TeamSize > 4 {
		return false
	}
	if p.PlayerTeam != 0 && p.PlayerTeam != 1 {
		return false
	}
	if p.TeamBlueScore < 0 || p.TeamOrangeScore < 0 {
		return false
	}
	if p.Goals < 0 || p.Assists < 0 || p.Saves < 0 ||
		p.Shots < 0 || p.Score < 0 || p.Demos < 0 {
		return false
	}
	if p.DurationSeconds < 0 || p.DurationSeconds > maxDuration {
		return false
	}
	// Le résultat annoncé doit correspondre aux scores : un client ne se
	// déclare pas vainqueur d'un match qu'il a perdu.
	if p.Result != expectedResult(p.PlayerTeam, p.TeamBlueScore, p.TeamOrangeScore) {
		return false
	}
	// Le MVP est le meilleur score de l'équipe GAGNANTE : il n'existe pas sans
	// victoire.
	if p.MVP && p.Result != "win" {
		return false
	}
	// Un match en file d'attente peut être vieux, jamais futur.
	if p.PlayedAt.After(time.Now().Add(clockSkewTolerance)) {
		return false
	}
	return true
}

// Recorder écrit les matchs Rocket League rapportés par le client de bureau.
type Recorder struct {
	st *store.Store
}

// NewRecorder construit l'enregistreur.
func NewRecorder(st *store.Store) *Recorder { return &Recorder{st: st} }

// Slug rend le slug de catalogue revendiqué.
func (r *Recorder) Slug() string { return "rocket-league" }

// Record valide puis écrit un match.
//
// Même enveloppe d'erreur que l'enregistreur Hearthstone : une seule sentinelle
// sur le fil, le détail dans le journal. playlist est le champ qui discrimine,
// puisqu'un identifiant inconnu de Psyonix est la raison la plus probable d'un
// refus légitime.
func (r *Recorder) Record(ctx context.Context, userID, gameID string, raw json.RawMessage) error {
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("%w: json illisible: %v", matchrecord.ErrInvalidPayload, err)
	}
	if !p.valid() {
		// La charge utile ENTIÈRE part au journal, et c'est sans danger par
		// construction : tous ses champs sont des nombres, des booléens ou une
		// énumération, par la même conception qui rend la table incapable de
		// porter un pseudonyme. Il n'y a rien de personnel à y fuiter.
		//
		// N'en nommer que deux obligerait l'exploitant à rejouer à la main
		// chaque règle de valid() sur une charge utile qui, elle, n'était pas
		// journalisée.
		return fmt.Errorf("%w: champs refusés (%+v)", matchrecord.ErrInvalidPayload, p)
	}
	return r.st.InsertRocketLeagueMatch(ctx, store.RocketLeagueMatch{
		UserID: userID, GameID: gameID,
		Playlist: p.Playlist, TeamSize: p.TeamSize, PlayerTeam: p.PlayerTeam,
		TeamBlueScore: p.TeamBlueScore, TeamOrangeScore: p.TeamOrangeScore,
		Result: p.Result,
		Goals:  p.Goals, Assists: p.Assists, Saves: p.Saves,
		Shots: p.Shots, Score: p.Score, Demos: p.Demos,
		MVP: p.MVP, DurationSeconds: p.DurationSeconds,
		PlayedAt: p.PlayedAt,
	})
}
