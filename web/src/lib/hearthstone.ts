import type { HsProfile, HsProfileHero, HsRecentMatch, HsPlayer } from './api';
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

/** How many places a Battlegrounds lobby has, and the window of the trend. */
const HS_PLACES = 8;
const HS_TREND_WINDOW = 20;

/**
 * The eight places of a Battlegrounds lobby, each with its match count and its
 * share of the member's placed matches.
 *
 * The server only sends the places actually reached, but a bar missing from the
 * chart reads as "no data" where it really means "never finished there", which
 * is exactly the information the spread is meant to show. So every place from 1
 * to 8 comes back, absent ones at zero.
 *
 * `share` is taken over the bars themselves rather than over the profile's
 * match count, so the eight shares always add up to one, and it is 0 when the
 * member has no placed match at all rather than a division by zero.
 */
export function hsPlacementBars(
	profile: HsProfile
): { placement: number; matches: number; share: number }[] {
	const counts = new Map<number, number>();
	for (const entry of profile.by_placement ?? []) {
		counts.set(entry.placement, (counts.get(entry.placement) ?? 0) + entry.matches);
	}
	let total = 0;
	for (let place = 1; place <= HS_PLACES; place++) total += counts.get(place) ?? 0;
	return Array.from({ length: HS_PLACES }, (_, i) => {
		const placement = i + 1;
		const matches = counts.get(placement) ?? 0;
		return { placement, matches, share: total > 0 ? matches / total : 0 };
	});
}

/**
 * Rolling average placement over a twenty match window, oldest point first so
 * the series can be drawn straight onto a left-to-right axis.
 *
 * `recent` arrives most recent first, so it is reversed before averaging.
 * Constructed games carry no placement and are skipped: they would otherwise
 * punch holes in a curve that only means something for Battlegrounds.
 *
 * Fewer than twenty placed matches yields an empty series on purpose: a rolling
 * average computed over less than its own window is not the same statistic, and
 * showing it would suggest a trend where there is only noise.
 */
export function hsTrend(recent: HsRecentMatch[]): number[] {
	const placements: number[] = [];
	for (const m of recent ?? []) {
		if (m.placement !== undefined) placements.push(m.placement);
	}
	placements.reverse(); // oldest first
	if (placements.length < HS_TREND_WINDOW) return [];
	const series: number[] = [];
	let sum = 0;
	for (let i = 0; i < placements.length; i++) {
		sum += placements[i];
		if (i >= HS_TREND_WINDOW) sum -= placements[i - HS_TREND_WINDOW];
		if (i >= HS_TREND_WINDOW - 1) series.push(sum / HS_TREND_WINDOW);
	}
	return series;
}

/**
 * A hero's top four rate for one member, as a whole percentage.
 *
 * The profile's hero list only holds placed matches, so the rate is taken over
 * every match of that hero. Zero matches yields 0 rather than a division by
 * zero.
 */
export function hsHeroRate(hero: HsProfileHero): number {
	return hero.matches > 0 ? Math.round((hero.top4 * 100) / hero.matches) : 0;
}
