import type { HsPlayer } from './api';
import { getLocale } from './i18n';
import heroes from './hs-heroes.json';

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

/**
 * Hero names by card identifier, French then English.
 *
 * Vendored rather than fetched: the public card catalogue weighs ten megabytes
 * for the thirty entries a page needs, and the names of heroes already released
 * never change. Regenerate it when Blizzard adds heroes; until then an unknown
 * identifier falls back to itself, which degrades one label instead of the
 * page.
 */
const HERO_NAMES: Record<string, string[]> = heroes;

/** A hero skin carries its own identifier; it dresses the hero before it. */
function baseHero(id: string): string {
	return id.replace(/_SKIN_[A-Za-z0-9]+$/, '');
}

/** The hero's name in the reader's language, falling back to its identifier. */
export function heroName(id: string): string {
	const entry = HERO_NAMES[id] ?? HERO_NAMES[baseHero(id)];
	if (!entry) return id;
	return (getLocale() === 'fr' ? entry[0] : entry[1]) || entry[1] || id;
}

/** The hero's illustration, served by the public card catalogue. */
export function heroArt(id: string): string {
	return `https://art.hearthstonejson.com/v1/256x/${baseHero(id)}.jpg`;
}
