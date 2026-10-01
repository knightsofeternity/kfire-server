import { describe, expect, it } from 'vitest';
import { playingGames } from './playing';

const wow = { id: '1', name: 'World of Warcraft', slug: 'world-of-warcraft' };
const mk = { id: '2', name: 'Mario Kart World', slug: 'mario-kart-world' };

describe('playingGames', () => {
	it('lists every game in progress, as the server orders them', () => {
		const games = playingGames({
			status: 'in_game',
			game: mk,
			games: [
				{ game: mk, since: '2026-10-01T18:00:00Z', platform: 'nintendo' },
				{ game: wow, since: '2026-10-01T17:00:00Z' }
			]
		});
		expect(games.map((g) => g.game.name)).toEqual(['Mario Kart World', 'World of Warcraft']);
		expect(games[0].platform).toBe('nintendo');
	});
	it('falls back to the single game of an older server', () => {
		expect(playingGames({ status: 'in_game', game: wow, platform: undefined })).toHaveLength(1);
	});
	it('is empty when not in game', () => {
		expect(playingGames({ status: 'online', game: null })).toEqual([]);
	});
});
