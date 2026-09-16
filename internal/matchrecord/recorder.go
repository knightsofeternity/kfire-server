// Package matchrecord aiguille un match terminé, rapporté par un client de
// bureau, vers le code qui sait le lire.
//
// Le plan de contrôle ne porte QU'UN message pour tous les jeux. Sans ce
// registre, le hub devrait connaître la charge utile de chaque jeu et gagnerait
// un cas de plus à chaque nouveau jeu, pour toujours.
package matchrecord

import (
	"context"
	"encoding/json"
	"errors"
)

// ErrUnknownGame est rendue quand aucun enregistreur ne revendique le slug.
var ErrUnknownGame = errors.New("aucun enregistreur pour ce jeu")

// ErrInvalidPayload est rendue par un enregistreur quand la charge utile ne
// mérite pas d'être écrite. Le hub la traduit en erreur visible par le client,
// jamais en erreur serveur.
var ErrInvalidPayload = errors.New("charge utile de match invalide")

// Recorder valide et persiste les résultats de match d'UN jeu.
type Recorder interface {
	// Slug est le slug de catalogue que cet enregistreur revendique.
	Slug() string

	// Record valide raw puis écrit un match pour userID dans gameID.
	// Rend ErrInvalidPayload quand raw n'est pas digne de confiance.
	Record(ctx context.Context, userID, gameID string, raw json.RawMessage) error
}

// Registry résout un slug vers son enregistreur.
//
// La map est construite une fois par NewRegistry et plus jamais écrite : il n'y
// a aucun mutateur à l'exécution, donc aucun verrou. C'est une raison PLUS forte
// que celle du registre de plugins, dont le verrou existe pour SetEnabled, que
// l'admin appelle longtemps après le démarrage.
//
// Non nil par construction, comme le registre de plugins : un récepteur nil ne
// peut venir que d'un serveur mal câblé, et il vaut mieux qu'il panique au
// premier match que de refuser silencieusement chaque match comme « jeu
// inconnu ».
type Registry struct {
	bySlug map[string]Recorder
}

// NewRegistry construit un registre à partir des enregistreurs donnés. Un
// enregistreur qui revendique un slug déjà pris remplace le précédent.
func NewRegistry(recorders ...Recorder) *Registry {
	m := make(map[string]Recorder, len(recorders))
	for _, r := range recorders {
		m[r.Slug()] = r
	}
	return &Registry{bySlug: m}
}

// forSlug rend l'enregistreur revendiquant slug, ou nil.
func (r *Registry) forSlug(slug string) Recorder {
	if slug == "" {
		return nil
	}
	return r.bySlug[slug]
}

// Record confie la charge utile à l'enregistreur revendiquant slug.
func (r *Registry) Record(ctx context.Context, slug, userID, gameID string, raw json.RawMessage) error {
	rec := r.forSlug(slug)
	if rec == nil {
		return ErrUnknownGame
	}
	return rec.Record(ctx, userID, gameID, raw)
}
