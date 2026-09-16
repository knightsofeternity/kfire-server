package matchrecord

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// fake is a test recorder that notes what it was passed.
type fake struct {
	slug   string
	err    error
	gotRaw string
	gotUID string
	gotGID string
}

func (f *fake) Slug() string { return f.slug }

func (f *fake) Record(_ context.Context, userID, gameID string, raw json.RawMessage) error {
	f.gotUID, f.gotGID, f.gotRaw = userID, gameID, string(raw)
	return f.err
}

func TestRegistryRoutesOnTheSlug(t *testing.T) {
	hs := &fake{slug: "hearthstone"}
	rl := &fake{slug: "rocket-league"}
	reg := NewRegistry(hs, rl)

	err := reg.Record(context.Background(), "rocket-league", "u1", "g1", json.RawMessage(`{"a":1}`))
	if err != nil {
		t.Fatalf("Record() = %v, want nil", err)
	}
	if rl.gotRaw != `{"a":1}` || rl.gotUID != "u1" || rl.gotGID != "g1" {
		t.Errorf("the wrong recorder received the payload: %+v", rl)
	}
	if hs.gotRaw != "" {
		t.Errorf("hearthstone received a match that is not its own")
	}
}

func TestRegistryUnknownSlug(t *testing.T) {
	reg := NewRegistry(&fake{slug: "hearthstone"})
	err := reg.Record(context.Background(), "minecraft", "u1", "g1", json.RawMessage(`{}`))
	if !errors.Is(err, ErrUnknownGame) {
		t.Errorf("Record() = %v, want ErrUnknownGame", err)
	}
}

func TestRegistryEmptySlug(t *testing.T) {
	reg := NewRegistry(&fake{slug: "hearthstone"})
	err := reg.Record(context.Background(), "", "u1", "g1", json.RawMessage(`{}`))
	if !errors.Is(err, ErrUnknownGame) {
		t.Errorf("Record() = %v, want ErrUnknownGame", err)
	}
}

func TestRegistryPropagatesTheRecorderError(t *testing.T) {
	boom := errors.New("boom")
	reg := NewRegistry(&fake{slug: "hearthstone", err: boom})
	err := reg.Record(context.Background(), "hearthstone", "u1", "g1", json.RawMessage(`{}`))
	if !errors.Is(err, boom) {
		t.Errorf("Record() = %v, want boom", err)
	}
}
