package gameplugin

import (
	"context"
	"sync"
	"testing"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

// fakePlugin is a test double; the display methods return a marker map.
type fakePlugin struct {
	id, name, conn string
	avail          bool
	slugs          []string
	refreshed      []string
}

func (f *fakePlugin) ID() string        { return f.id }
func (f *fakePlugin) Name() string      { return f.name }
func (f *fakePlugin) Connector() string { return f.conn }
func (f *fakePlugin) Available() bool   { return f.avail }
func (f *fakePlugin) Slugs() []string   { return f.slugs }
func (f *fakePlugin) Refresh(_ context.Context, _, slug string) {
	f.refreshed = append(f.refreshed, slug)
}
func (f *fakePlugin) GameDetail(_ context.Context, _ string, _ store.Game) (map[string]any, error) {
	return map[string]any{f.id + "_agg": true}, nil
}
func (f *fakePlugin) UserGameDetail(_ context.Context, _ string, _ store.Game) (map[string]any, error) {
	return map[string]any{f.id + "_user": true}, nil
}

// fakeStore implements the registry's stateStore.
type fakeStore struct{ states map[string]bool }

func (s *fakeStore) PluginStates(_ context.Context) (map[string]bool, error) {
	return s.states, nil
}
func (s *fakeStore) SetPluginEnabled(_ context.Context, id string, enabled bool) error {
	s.states[id] = enabled
	return nil
}
func (s *fakeStore) EnsurePluginDefaults(_ context.Context, ids []string) error {
	for _, id := range ids {
		if _, ok := s.states[id]; !ok {
			s.states[id] = true
		}
	}
	return nil
}

func TestRegistryForSlugReturnsActiveOnly(t *testing.T) {
	wow := &fakePlugin{id: "wow", avail: true, slugs: []string{"world-of-warcraft"}}
	d3 := &fakePlugin{id: "d3", avail: true, slugs: []string{"diablo-iii"}}
	lol := &fakePlugin{id: "lol", avail: false, slugs: []string{"league-of-legends"}} // unavailable

	st := &fakeStore{states: map[string]bool{"wow": true, "d3": false}} // d3 disabled
	r := NewRegistry(st)
	r.Register(wow)
	r.Register(d3)
	r.Register(lol)
	if err := r.Load(context.Background()); err != nil {
		t.Fatalf("load: %v", err)
	}

	if got := r.ForSlug("world-of-warcraft"); len(got) != 1 || got[0].ID() != "wow" {
		t.Fatalf("wow should be active for its slug, got %v", got)
	}
	if got := r.ForSlug("diablo-iii"); len(got) != 0 {
		t.Fatalf("d3 is disabled, expected none, got %v", got)
	}
	if got := r.ForSlug("league-of-legends"); len(got) != 0 {
		t.Fatalf("lol is unavailable, expected none, got %v", got)
	}
}

func TestRegistryLoadSeedsDefaultEnabled(t *testing.T) {
	// "newcomer" is registered but absent from the store -> defaults to enabled.
	p := &fakePlugin{id: "newcomer", avail: true, slugs: []string{"some-game"}}
	st := &fakeStore{states: map[string]bool{}}
	r := NewRegistry(st)
	r.Register(p)
	if err := r.Load(context.Background()); err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := r.ForSlug("some-game"); len(got) != 1 {
		t.Fatalf("newcomer should default to enabled, got %v", got)
	}
}

func TestRegistryListReportsAvailabilityAndEnabled(t *testing.T) {
	wow := &fakePlugin{id: "wow", name: "WoW", conn: "battlenet", avail: true, slugs: []string{"world-of-warcraft"}}
	lol := &fakePlugin{id: "lol", name: "LoL", conn: "riot", avail: false, slugs: []string{"league-of-legends"}}
	st := &fakeStore{states: map[string]bool{"wow": true, "lol": true}}
	r := NewRegistry(st)
	r.Register(wow)
	r.Register(lol)
	if err := r.Load(context.Background()); err != nil {
		t.Fatalf("load: %v", err)
	}
	list := r.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list))
	}
	if list[0].ID != "wow" || !list[0].Available || !list[0].Enabled {
		t.Fatalf("wow entry wrong: %+v", list[0])
	}
	if list[1].ID != "lol" || list[1].Available || !list[1].Enabled {
		t.Fatalf("lol entry should be unavailable+enabled: %+v", list[1])
	}
}

func TestRegistryActiveListsSlugs(t *testing.T) {
	wow := &fakePlugin{id: "wow", avail: true, slugs: []string{"world-of-warcraft", "world-of-warcraft-classic"}}
	st := &fakeStore{states: map[string]bool{"wow": true}}
	r := NewRegistry(st)
	r.Register(wow)
	if err := r.Load(context.Background()); err != nil {
		t.Fatalf("load: %v", err)
	}
	active := r.Active()
	if len(active) != 1 || active[0].ID != "wow" || len(active[0].Slugs) != 2 {
		t.Fatalf("active wrong: %+v", active)
	}
}

func TestRegistrySetEnabledUnknownPlugin(t *testing.T) {
	st := &fakeStore{states: map[string]bool{}}
	r := NewRegistry(st)
	if err := r.SetEnabled(context.Background(), "ghost", true); err != ErrUnknownPlugin {
		t.Fatalf("expected ErrUnknownPlugin, got %v", err)
	}
}

func TestRegistrySetEnabledTogglesImmediately(t *testing.T) {
	wow := &fakePlugin{id: "wow", avail: true, slugs: []string{"world-of-warcraft"}}
	st := &fakeStore{states: map[string]bool{"wow": true}}
	r := NewRegistry(st)
	r.Register(wow)
	if err := r.Load(context.Background()); err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := r.SetEnabled(context.Background(), "wow", false); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := r.ForSlug("world-of-warcraft"); len(got) != 0 {
		t.Fatalf("wow should be off immediately, got %v", got)
	}
}

// lockedStore is a stateStore whose map is guarded, so the concurrency test
// exercises the Registry's RWMutex rather than a data race inside the fake.
type lockedStore struct {
	mu     sync.Mutex
	states map[string]bool
}

func (s *lockedStore) PluginStates(_ context.Context) (map[string]bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]bool, len(s.states))
	for k, v := range s.states {
		out[k] = v
	}
	return out, nil
}
func (s *lockedStore) SetPluginEnabled(_ context.Context, id string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[id] = enabled
	return nil
}
func (s *lockedStore) EnsurePluginDefaults(_ context.Context, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range ids {
		if _, ok := s.states[id]; !ok {
			s.states[id] = true
		}
	}
	return nil
}

// TestRegistryConcurrentAccess registers before Load (honoring the startup
// contract), then fires concurrent ForSlug + List readers alongside SetEnabled
// runtime mutations to exercise the RWMutex under the race detector.
func TestRegistryConcurrentAccess(t *testing.T) {
	wow := &fakePlugin{id: "wow", name: "WoW", conn: "battlenet", avail: true, slugs: []string{"world-of-warcraft"}}
	d3 := &fakePlugin{id: "d3", name: "D3", conn: "battlenet", avail: true, slugs: []string{"diablo-iii"}}
	st := &lockedStore{states: map[string]bool{"wow": true, "d3": true}}
	r := NewRegistry(st)
	r.Register(wow)
	r.Register(d3)
	if err := r.Load(context.Background()); err != nil {
		t.Fatalf("load: %v", err)
	}

	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = r.ForSlug("world-of-warcraft")
				_ = r.List()
				_ = r.Active()
				if j%5 == 0 {
					_ = r.SetEnabled(ctx, "d3", j%2 == 0)
				}
			}
		}(i)
	}
	wg.Wait()
}
