// Package matchrecord routes a finished match, reported by a desktop client,
// to the code that knows how to read it.
//
// The control plane carries only ONE message for every game. Without this
// registry, the hub would need to know each game's payload and would gain
// one more case with every new game, forever.
package matchrecord

import (
	"context"
	"encoding/json"
	"errors"
)

// ErrUnknownGame is returned when no recorder claims the slug.
var ErrUnknownGame = errors.New("no recorder for this game")

// ErrInvalidPayload is returned by a recorder when the payload does not
// deserve to be written. The hub translates it into an error visible to the
// client, never a server error.
var ErrInvalidPayload = errors.New("invalid match payload")

// Recorder validates and persists the match results of ONE game.
type Recorder interface {
	// Slug is the catalog slug this recorder claims.
	Slug() string

	// Record validates raw then writes a match for userID in gameID.
	// Returns ErrInvalidPayload when raw is not trustworthy.
	Record(ctx context.Context, userID, gameID string, raw json.RawMessage) error
}

// Registry resolves a slug to its recorder.
//
// The map is built once by NewRegistry and never written again: there is no
// mutator at runtime, hence no lock. That is a STRONGER reason than the one
// behind the plugin registry's lock, which exists for SetEnabled, called by
// the admin long after startup.
//
// Non-nil by construction, like the plugin registry: a nil receiver can only
// come from a badly wired server, and it is better for it to panic on the
// first match than to silently reject every match as "unknown game".
type Registry struct {
	bySlug map[string]Recorder
}

// NewRegistry builds a registry from the given recorders. A recorder that
// claims a slug already taken replaces the previous one.
func NewRegistry(recorders ...Recorder) *Registry {
	m := make(map[string]Recorder, len(recorders))
	for _, r := range recorders {
		m[r.Slug()] = r
	}
	return &Registry{bySlug: m}
}

// forSlug returns the recorder claiming slug, or nil.
func (r *Registry) forSlug(slug string) Recorder {
	if slug == "" {
		return nil
	}
	return r.bySlug[slug]
}

// Record hands the payload to the recorder claiming slug.
func (r *Registry) Record(ctx context.Context, slug, userID, gameID string, raw json.RawMessage) error {
	rec := r.forSlug(slug)
	if rec == nil {
		return ErrUnknownGame
	}
	return rec.Record(ctx, userID, gameID, raw)
}
