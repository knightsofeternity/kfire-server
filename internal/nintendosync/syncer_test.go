package nintendosync

import (
	"testing"

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
