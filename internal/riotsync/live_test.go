package riotsync

import (
	"testing"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/connectors/riot"
)

func TestLiveRegistrySetAndGet(t *testing.T) {
	r := newLiveRegistry()
	r.set("u1", &riot.LiveGame{ChampionID: 202, Mode: "ARAM"})

	got := r.get("u1")
	if got == nil || got.ChampionID != 202 {
		t.Fatalf("get = %+v, want champion 202", got)
	}
	if r.get("nobody") != nil {
		t.Error("an unknown member must return nil")
	}
}

func TestLiveRegistryClearRemovesTheEntry(t *testing.T) {
	r := newLiveRegistry()
	r.set("u1", &riot.LiveGame{ChampionID: 1})
	r.clear("u1")
	if r.get("u1") != nil {
		t.Error("clear must remove the entry")
	}
}

func TestLiveRegistryEntryExpires(t *testing.T) {
	r := newLiveRegistry()
	r.set("u1", &riot.LiveGame{ChampionID: 1})
	// Age the entry past the TTL without sleeping.
	r.mu.Lock()
	e := r.entries["u1"]
	e.storedAt = time.Now().Add(-liveTTL - time.Second)
	r.entries["u1"] = e
	r.mu.Unlock()

	if r.get("u1") != nil {
		t.Error("an entry older than the TTL must read as absent")
	}
}

func TestLiveRegistrySetNilClears(t *testing.T) {
	r := newLiveRegistry()
	r.set("u1", &riot.LiveGame{ChampionID: 1})
	r.set("u1", nil)
	if r.get("u1") != nil {
		t.Error("setting nil must clear the entry, so leaving a game is visible at once")
	}
}

func TestLiveRegistryIsSafeUnderConcurrentUse(t *testing.T) {
	r := newLiveRegistry()
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func(i int) {
			defer func() { done <- struct{}{} }()
			for n := 0; n < 50; n++ {
				r.set("u1", &riot.LiveGame{ChampionID: i})
				_ = r.get("u1")
				r.clear("u2")
			}
		}(i)
	}
	for i := 0; i < 8; i++ {
		<-done
	}
}

func TestLiveGameOnASyncerWithoutARegistryReturnsNil(t *testing.T) {
	var s Syncer // zero value: no registry
	if s.LiveGame("u1") != nil {
		t.Error("a syncer built without a registry must not panic and must return nil")
	}
}

func TestPluginActiveDefaultsToTrueWithoutACheck(t *testing.T) {
	s := New(nil, nil, nil)
	if !s.pluginActive() {
		t.Error("a syncer wired without a check must behave as active, " +
			"so the lazy refresh path keeps working")
	}
}

func TestPluginActiveFollowsTheCheck(t *testing.T) {
	s := New(nil, nil, nil)
	on := true
	s.SetActiveCheck(func() bool { return on })

	if !s.pluginActive() {
		t.Error("want active while the check says so")
	}
	on = false
	if s.pluginActive() {
		t.Error("the admin turning the plugin off must stop the live loop, " +
			"otherwise it keeps calling Riot every minute for a disabled plugin")
	}
}
