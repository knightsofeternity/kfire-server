package store

// Playtime merge model (per user, per game).
//
// We have two sources: locally observed KFIRE sessions and imported platform
// lifetime playtime (Steam `playtime_forever`). Naively summing them would
// double-count time played through Steam while the KFIRE client is running, and
// naively taking the max hides recent play until the local cumulative exceeds
// the whole imported lifetime.
//
// Instead we keep the synced figure as a **baseline** and add only the local
// sessions recorded **since** that sync. At each sync we move the baseline to
// the larger of the platform's new lifetime and what we were already showing,
// so recent play appears immediately, Steam-tracked time is never doubled, and
// local play the platform never observed is never lost.

// effectivePlaytimeSeconds returns the displayed playtime for one game. With a
// baseline (a Steam-linked game), it is the baseline plus local sessions since
// the last sync; otherwise it is all local sessions.
func effectivePlaytimeSeconds(hasBaseline bool, baseline, localSinceSync, localTotal int64) int64 {
	if hasBaseline {
		return baseline + localSinceSync
	}
	return localTotal
}

// syncedBaseline computes the baseline to store at sync time: the larger of the
// platform's reported lifetime and the figure we were already displaying
// (previous baseline + local sessions since the previous sync).
func syncedBaseline(platformSeconds, oldBaseline, localSincePrevSync int64) int64 {
	estimated := oldBaseline + localSincePrevSync
	if platformSeconds > estimated {
		return platformSeconds
	}
	return estimated
}
