import type { HsPlayer } from './api';

/**
 * Win rate across every reported match, as a whole percentage.
 * Zero matches yields 0 rather than a division by zero.
 */
export function hsWinRate(p: HsPlayer): number {
	return p.matches > 0 ? Math.round((p.wins * 100) / p.matches) : 0;
}

/**
 * Share of Battlegrounds matches finished in the top four, as a whole
 * percentage. Computed over matches that carry a placement, never over every
 * match: a constructed game has no placement and would dilute the figure.
 */
export function hsTop4Rate(p: HsPlayer): number {
	return p.ranked > 0 ? Math.round((p.top4 * 100) / p.ranked) : 0;
}

/**
 * Members ordered by average placement, best first, then by match count.
 *
 * Average placement is the guild's measure of Battlegrounds skill, because the
 * rating itself is NOT recoverable: it appears nowhere in the game's logs, and
 * every tracker that shows it queries an external leaderboard covering only
 * players above 8000.
 *
 * Members without a single placement sort last: they have nothing to compare.
 */
export function hsByPlacement(players: HsPlayer[]): HsPlayer[] {
	return [...players].sort((a, b) => {
		const pa = a.avg_placement ?? Infinity;
		const pb = b.avg_placement ?? Infinity;
		if (pa !== pb) return pa - pb;
		return b.matches - a.matches || a.username.localeCompare(b.username);
	});
}

/** Total matches reported by the guild. */
export function hsTotalMatches(players: HsPlayer[]): number {
	return players.reduce((n, p) => n + p.matches, 0);
}
