package ws

import "time"

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
	if p.GameSlug == "" {
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
	if p.Goals < 0 || p.Assists < 0 || p.Saves < 0 ||
		p.Shots < 0 || p.Score < 0 || p.Demos < 0 {
		return false
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
