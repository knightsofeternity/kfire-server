import type { HsProfile, HsProfileHero, HsRecentMatch, HsPlayer } from './api';
import type { LiveEntry } from './stores/live.svelte';
import { getLocale, t } from './i18n';
import heroes from './hs-heroes.json';

/** The slug the server uses for Hearthstone on recaps and presence. */
export const HS_SLUG = 'hearthstone';

/**
 * A Hearthstone game in progress, as the desktop client reports it.
 *
 * Deliberately this short, and not a subset of anything: the game's log names
 * the opponent and lists every card played, and none of that is broadcast. The
 * mode is one of the two the server accepts, never a free string.
 *
 * `placement` only exists in Battlegrounds; a constructed game carries no such
 * key at all, which is why it is optional rather than nullable.
 */
export type HsLiveMatch = {
	mode: string;
	turn: number;
	placement?: number;
};

/**
 * The Hearthstone reading of a live entry, or `null` for anything else.
 *
 * The socket carries no type information beyond `game_slug`, so this is the
 * single place where a Hearthstone live payload is asserted into a shape, and
 * it happens only once the slug has been checked at runtime.
 */
export function hsLiveMatch(entry: LiveEntry | undefined): HsLiveMatch | null {
	if (!entry || entry.game_slug !== HS_SLUG) return null;
	return entry.match as HsLiveMatch;
}

/** Mode labels the catalog knows; anything else shows the raw mode name. */
const HS_MODE_KEYS: Record<string, string> = {
	battlegrounds: 'game.hsModeBattlegrounds',
	constructed: 'game.hsModeConstructed',
	arena: 'game.hsModeArena'
};

/**
 * The mode's label in the reader's language. An unknown mode falls back to its
 * raw name, which degrades one label instead of hiding the match.
 */
export function hsModeLabel(mode: string): string {
	const key = HS_MODE_KEYS[mode];
	return key ? t(key) : mode;
}

/** The Battlegrounds mode, the only one that finishes on a placement. */
const HS_BATTLEGROUNDS = 'battlegrounds';

/** The places players themselves count as a good game, the game's own top 4. */
const HS_TOP_PLACES = 4;

/** How one match reads on screen: its label, its accent, and the crown. */
export type HsResultInfo = {
	label: string;
	colorClass: string;
	/** First place only: a lobby of eight has exactly one winner. */
	crown: boolean;
};

/**
 * How a finished Hearthstone match reads on screen.
 *
 * Battlegrounds has no win or loss, it has a place out of eight, and the game's
 * own PLAYSTATE says WON for a first place only. That is why a top 2 used to
 * show up as a red "Defeat": the stored result is exact, it just is not what
 * the mode means. In Battlegrounds the placement IS the result, and it carries
 * three levels rather than two, because finishing first and finishing fourth
 * are not the same thing to the player who did it.
 *
 * Constructed keeps win, loss and draw: there the words mean something.
 *
 * A Battlegrounds match whose log carried no placement falls back to the stored
 * result, imprecise but never empty: the game's log is not always complete.
 */
export function hsResultInfo(
	mode: string,
	result: string,
	placement?: number | null
): HsResultInfo {
	if (mode === HS_BATTLEGROUNDS && typeof placement === 'number') {
		return {
			label: t('game.hsTop', { n: placement }),
			colorClass:
				placement === 1
					? 'text-[var(--color-gold)]'
					: placement <= HS_TOP_PLACES
						? 'text-[var(--color-online)]'
						: 'text-[var(--color-magenta)]',
			crown: placement === 1
		};
	}
	if (result === 'win')
		return { label: t('game.hsWin'), colorClass: 'text-[var(--color-online)]', crown: false };
	if (result === 'loss')
		return { label: t('game.hsLoss'), colorClass: 'text-[var(--color-magenta)]', crown: false };
	return { label: t('game.hsDraw'), colorClass: 'text-[var(--color-muted)]', crown: false };
}

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

/** Where members get Hearthstone Deck Tracker, the only source of the rating. */
export const HDT_URL = 'https://hsreplay.net/downloads/';

/** How many rated matches the rating curve spans. */
const HS_RATING_WINDOW = 20;

/**
 * Whether a member's rating is older than their last match: they played since
 * without HDT recording it. The page then dates the rating, so an old figure
 * never reads as today's.
 */
export function hsRatingStale(p: { rating_at?: string; last_played_at: string }): boolean {
	if (!p.rating_at) return false;
	return new Date(p.rating_at).getTime() < new Date(p.last_played_at).getTime();
}

/** Members by rating, highest first; members without one last, by name. */
export function hsByRating(players: HsPlayer[]): HsPlayer[] {
	return [...players].sort((a, b) => {
		const ra = a.rating ?? -1;
		const rb = b.rating ?? -1;
		if (ra !== rb) return rb - ra;
		return a.username.localeCompare(b.username);
	});
}

/**
 * The rating after each rated match, oldest first, over the last twenty rated
 * matches. Matches without a rating are skipped, never counted as zero.
 */
export function hsRatingSeries(recent: HsRecentMatch[]): number[] {
	const out: number[] = [];
	for (const m of recent ?? []) {
		if (typeof m.rating_after === 'number') out.push(m.rating_after);
		if (out.length === HS_RATING_WINDOW) break;
	}
	return out.reverse();
}

/** What one match did to the rating, or null when HDT did not record both ends. */
export function hsRatingDelta(m: HsRecentMatch): number | null {
	return typeof m.rating === 'number' && typeof m.rating_after === 'number'
		? m.rating_after - m.rating
		: null;
}
