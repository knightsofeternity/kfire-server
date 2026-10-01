import type { Game, PresenceEntry } from './api';

/** One game a member is playing right now. */
export type PlayingGame = { game: Game; since?: string; platform?: 'playstation' | 'xbox' | 'nintendo' };

/**
 * Every game a member is playing, newest first. A member can play on PC and
 * on a console at once; a server older than the `games` list only sends the
 * newest one, which is then the whole list.
 */
export function playingGames(entry: Pick<PresenceEntry, 'status' | 'game' | 'since' | 'platform' | 'games'>): PlayingGame[] {
	if (entry.status !== 'in_game') return [];
	if (entry.games && entry.games.length > 0) return entry.games;
	return entry.game ? [{ game: entry.game, since: entry.since, platform: entry.platform }] : [];
}
