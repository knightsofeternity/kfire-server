package nintendosync

import (
	"testing"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/connectors/nintendo"
)

func TestToAcceptLeavesStrangersAlone(t *testing.T) {
	req := func(id, nsa string) nintendo.FriendRequest {
		r := nintendo.FriendRequest{ID: id}
		r.Sender.NsaID = nsa
		return r
	}
	got := toAccept([]nintendo.FriendRequest{req("r1", "stranger"), req("r2", "member")}, map[string]string{"member": "u1"})
	if len(got) != 1 || got[0].ID != "r2" {
		t.Fatalf("got %+v: only a linked member's request is accepted", got)
	}
}

func TestNextPauseDoublesUpToACeiling(t *testing.T) {
	if got := nextPause(0, false); got != firstPause {
		t.Fatalf("first failure: %v", got)
	}
	if got := nextPause(firstPause, false); got != 2*firstPause {
		t.Fatalf("second failure: %v", got)
	}
	if got := nextPause(maxPause, false); got != maxPause {
		t.Fatalf("ceiling: %v", got)
	}
	if got := nextPause(0, true); got != rateLimitedPause {
		t.Fatalf("a refusal waits at least %v: %v", rateLimitedPause, got)
	}
}

func TestFailedPassPausesThePoller(t *testing.T) {
	now := time.Date(2026, 10, 29, 5, 0, 0, 0, time.UTC)
	s := &Syncer{now: func() time.Time { return now }}
	if !s.failed(t.Context(), "friend requests", nintendo.ErrRateLimited) {
		t.Fatal("an error stops the pass")
	}
	if !s.pausedUntil.Equal(now.Add(rateLimitedPause)) {
		t.Fatalf("paused until %v", s.pausedUntil)
	}
	// Tick returns before touching the bot (nil here) while paused.
	s.Tick(t.Context())
}
