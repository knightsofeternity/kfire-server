package bnetsync

import (
	"context"
	"testing"

	"github.com/knightsofeternity/kfire-server/internal/connectors/battlenet"
)

// When the Battle.net connector has no credentials configured, the on-view
// syncer must short-circuit before touching the store, so a disabled instance
// never queries the DB or calls Blizzard. A nil store proves the early return:
// any store access would panic on the nil pointer.
func TestRefreshNoopWhenConnectorDisabled(t *testing.T) {
	s := New(nil, battlenet.New("", ""), nil, "eu")

	// Neither call may panic; reaching s.store would deref nil.
	s.RefreshWoW(context.Background(), "user-1", "world-of-warcraft")
	s.RefreshBnetGame(context.Background(), "user-1", "diablo-iii")
}
