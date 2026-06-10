package store

import "testing"

func TestEffectivePlaytimeSeconds(t *testing.T) {
	// No baseline (non-Steam game): all local sessions count.
	if got := effectivePlaytimeSeconds(false, 0, 0, 300); got != 300 {
		t.Errorf("no baseline: got %d, want 300", got)
	}
	// Baseline present: baseline + local since last sync (not the full local total).
	if got := effectivePlaytimeSeconds(true, 360000, 1800, 999999); got != 361800 {
		t.Errorf("with baseline: got %d, want 361800", got)
	}
	// Baseline, nothing played since the sync.
	if got := effectivePlaytimeSeconds(true, 360000, 0, 0); got != 360000 {
		t.Errorf("baseline only: got %d, want 360000", got)
	}
}

func TestSyncedBaseline(t *testing.T) {
	// Steam-launched game: Steam already counted the local play, so the new
	// platform figure (105h) wins over baseline+local (100h+5h=105h) - a tie,
	// either way 105h, never 110h.
	if got := syncedBaseline(378000, 360000, 18000); got != 378000 {
		t.Errorf("steam-launched: got %d, want 378000", got)
	}
	// Non-Steam launcher: Steam figure unchanged (100h) but we played 5h locally,
	// so the estimated 105h wins and the local play is preserved on resync.
	if got := syncedBaseline(360000, 360000, 18000); got != 378000 {
		t.Errorf("non-steam play preserved: got %d, want 378000", got)
	}
	// First sync (no prior baseline, no local): the platform figure stands.
	if got := syncedBaseline(360000, 0, 0); got != 360000 {
		t.Errorf("first sync: got %d, want 360000", got)
	}
}
