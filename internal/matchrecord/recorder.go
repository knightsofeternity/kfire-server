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
// Construit une seule fois au démarrage puis lu seulement, donc sans verrou :
// même contrat que le registre de plugins, dont Load n'est appelé qu'une fois.
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

// ForSlug rend l'enregistreur revendiquant slug, ou nil. Tolère un registre nil
// pour qu'un serveur mal câblé refuse les matchs au lieu de paniquer.
func (r *Registry) ForSlug(slug string) Recorder {
	if r == nil || slug == "" {
		return nil
	}
	return r.bySlug[slug]
}

// Record confie la charge utile à l'enregistreur revendiquant slug.
func (r *Registry) Record(ctx context.Context, slug, userID, gameID string, raw json.RawMessage) error {
	rec := r.ForSlug(slug)
	if rec == nil {
		return ErrUnknownGame
	}
	return rec.Record(ctx, userID, gameID, raw)
}
