package ws

import (
	"regexp"
	"time"
)

// liveTTL est la durée au bout de laquelle un état de match sans nouvelle est
// considéré comme terminé. Le client émet à 2 Hz, donc 15 secondes laissent une
// marge confortable à une connexion qui hoquette.
const liveTTL = 15 * time.Second

// maxMatchScore borne un score d'équipe. Rocket League n'a pas de limite dure,
// mais un match à plus de 99 buts n'existe pas : la borne empêche un client
// hostile de diffuser n'importe quoi à toute la guilde.
const maxMatchScore = 99

// maxSecondsRemaining borne le chrono affiché, prolongation comprise.
const maxSecondsRemaining = 7200

// maxStat borne les statistiques du membre. Large exprès : ce n'est pas une
// règle de jeu, c'est un garde-fou contre une valeur absurde diffusée à tout le
// monde. Les scores d'équipe et le chrono sont déjà bornés, ceux-ci ne l'étaient
// pas, sans raison.
const maxStat = 100000

// slugPattern est la forme d'un slug de catalogue.
//
// Ce message est le SEUL de toute la fonctionnalité dont le contenu est
// retransmis à d'autres membres. Le reste finit en base, protégé par des
// contraintes. Ici, une chaîne libre partirait telle quelle vers tous les
// navigateurs de la guilde, donc elle est bornée en longueur ET en alphabet :
// un slug ne peut contenir ni balise, ni espace, ni dix mégaoctets.
//
// Résoudre le slug dans le catalogue serait plus strict, mais ce message arrive
// deux fois par seconde et par joueur : frapper la base à ce rythme pour
// valider une constante n'aurait aucun sens.
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// livePayload est l'état courant d'un match, diffusé et JAMAIS écrit.
//
// Comme le résumé de fin de match, il ne nomme personne : deux scores d'équipe,
// un chrono, et les stats du membre. Le flux du jeu porte le nom de tous les
// joueurs, ils ne quittent pas sa machine, y compris pour un affichage.
type livePayload struct {
	GameSlug string `json:"game_slug"`
	// Ended marque la fin du match. Tous les autres champs sont alors ignorés.
	Ended            bool `json:"ended"`
	TeamBlueScore    int  `json:"team_blue_score"`
	TeamOrangeScore  int  `json:"team_orange_score"`
	SecondsRemaining int  `json:"seconds_remaining"`
	Overtime         bool `json:"overtime"`
	Goals            int  `json:"goals"`
	Assists          int  `json:"assists"`
	Saves            int  `json:"saves"`
	Shots            int  `json:"shots"`
	Score            int  `json:"score"`
	Demos            int  `json:"demos"`
}

// valid dit si l'état mérite d'être rediffusé. Il part vers tous les clients de
// l'org, donc il est validé avec la même rigueur qu'une donnée écrite.
func (p livePayload) valid() bool {
	if !slugPattern.MatchString(p.GameSlug) {
		return false
	}
	if p.Ended {
		return true
	}
	if p.TeamBlueScore < 0 || p.TeamBlueScore > maxMatchScore ||
		p.TeamOrangeScore < 0 || p.TeamOrangeScore > maxMatchScore {
		return false
	}
	if p.SecondsRemaining < 0 || p.SecondsRemaining > maxSecondsRemaining {
		return false
	}
	for _, v := range []int{p.Goals, p.Assists, p.Saves, p.Shots, p.Score, p.Demos} {
		if v < 0 || v > maxStat {
			return false
		}
	}
	return true
}

// liveEntry est l'état retenu en mémoire pour un membre.
type liveEntry struct {
	payload   livePayload
	updatedAt time.Time
}

// expired dit si l'entrée n'a plus reçu de nouvelle depuis assez longtemps pour
// être tenue pour finie.
func (e liveEntry) expired(now time.Time) bool {
	return now.Sub(e.updatedAt) > liveTTL
}
