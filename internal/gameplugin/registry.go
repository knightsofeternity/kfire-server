package gameplugin

import (
	"context"
	"errors"
	"sync"
)

// ErrUnknownPlugin is returned by SetEnabled for an id that was never registered.
var ErrUnknownPlugin = errors.New("unknown plugin")

// stateStore is the slice of *store.Store the registry needs (interface so the
// registry is unit-testable with a fake).
type stateStore interface {
	PluginStates(ctx context.Context) (map[string]bool, error)
	SetPluginEnabled(ctx context.Context, id string, enabled bool) error
	EnsurePluginDefaults(ctx context.Context, ids []string) error
}

// Registry holds the registered plugins and their cached enabled state.
type Registry struct {
	st      stateStore
	mu      sync.RWMutex
	order   []Plugin        // registration order
	enabled map[string]bool // id -> enabled (in-memory cache)
}

// NewRegistry builds an empty registry backed by st.
func NewRegistry(st stateStore) *Registry {
	return &Registry{st: st, enabled: map[string]bool{}}
}

// Register adds a plugin. Tolerant of an unavailable connector: the plugin is
// registered but never active until Available() returns true.
func (r *Registry) Register(p Plugin) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.order = append(r.order, p)
}

// Load seeds default-enabled rows for any registered plugin missing from the
// DB, then fills the in-memory enabled cache.
//
// Concurrency contract: Load is intended to be called ONCE at startup, AFTER
// all Register calls have completed, and must NOT run concurrently with
// SetEnabled — it swaps the whole enabled map under the write lock, so a
// concurrent runtime mutation would be lost on the swap.
func (r *Registry) Load(ctx context.Context) error {
	r.mu.RLock()
	ids := make([]string, len(r.order))
	for i, p := range r.order {
		ids[i] = p.ID()
	}
	r.mu.RUnlock()

	if err := r.st.EnsurePluginDefaults(ctx, ids); err != nil {
		return err
	}
	states, err := r.st.PluginStates(ctx)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.enabled = states
	r.mu.Unlock()
	return nil
}

// isActive assumes the caller holds at least an RLock. Plugin.Available() must
// be safe for concurrent calls, since it runs under the read lock alongside
// other readers.
func (r *Registry) isActive(p Plugin) bool {
	return p.Available() && r.enabled[p.ID()]
}

// ForSlug returns the active plugins owning slug (used by handlers). It returns
// nil when no active plugin owns the slug; callers range over the result, and
// it is never JSON-serialized directly (unlike Active, which returns a non-nil
// slice).
func (r *Registry) ForSlug(slug string) []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Plugin
	for _, p := range r.order {
		if !r.isActive(p) {
			continue
		}
		for _, s := range p.Slugs() {
			if s == slug {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// List returns every registered plugin with its availability + enabled flag.
func (r *Registry) List() []Info {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Info, len(r.order))
	for i, p := range r.order {
		out[i] = Info{
			ID:        p.ID(),
			Name:      p.Name(),
			Connector: p.Connector(),
			Available: p.Available(),
			Enabled:   r.enabled[p.ID()],
		}
	}
	return out
}

// Active returns the active plugins with their slugs (for public config).
func (r *Registry) Active() []ActivePlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []ActivePlugin{}
	for _, p := range r.order {
		if r.isActive(p) {
			out = append(out, ActivePlugin{ID: p.ID(), Slugs: p.Slugs()})
		}
	}
	return out
}

// ActivePlugins returns every currently active plugin (available + enabled),
// for callers that must act on each active plugin (e.g. warming a member's data
// across all enabled games). Returns nil when none are active.
func (r *Registry) ActivePlugins() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Plugin
	for _, p := range r.order {
		if r.isActive(p) {
			out = append(out, p)
		}
	}
	return out
}

// SetEnabled persists and caches a plugin's enabled flag (immediate effect).
//
// Concurrency contract: this is the only intended RUNTIME mutator of enabled
// state (the admin toggle). It updates a single map key under the write lock
// and relies on the Load-at-startup contract above — Load must not run
// concurrently with it.
func (r *Registry) SetEnabled(ctx context.Context, id string, enabled bool) error {
	r.mu.RLock()
	known := false
	for _, p := range r.order {
		if p.ID() == id {
			known = true
			break
		}
	}
	r.mu.RUnlock()
	if !known {
		return ErrUnknownPlugin
	}
	if err := r.st.SetPluginEnabled(ctx, id, enabled); err != nil {
		return err
	}
	r.mu.Lock()
	r.enabled[id] = enabled
	r.mu.Unlock()
	return nil
}
