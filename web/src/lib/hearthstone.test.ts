import { describe, expect, it } from 'vitest';
import { hsByRating, hsRatingSeries, hsRatingStale, hsRatingDelta } from './hearthstone';
import type { HsPlayer, HsRecentMatch } from './api';

const player = (p: Partial<HsPlayer>): HsPlayer => ({
	user_id: p.username ?? 'x', username: 'x', matches: 1, wins: 0, ranked: 1, top4: 0,
	last_played_at: '2026-09-21T12:00:00Z', ...p
});

describe('hsRatingStale', () => {
	it('is fresh when the rating comes from the last Battlegrounds match', () => {
		expect(hsRatingStale({ rating_at: '2026-09-21T12:00:00Z', last_bg_played_at: '2026-09-21T12:00:00Z' })).toBe(false);
	});
	it('is stale when later Battlegrounds matches carried no rating', () => {
		expect(hsRatingStale({ rating_at: '2026-09-20T12:00:00Z', last_bg_played_at: '2026-09-21T12:00:00Z' })).toBe(true);
	});
	it('is not stale without any rating', () => {
		expect(hsRatingStale({ last_bg_played_at: '2026-09-21T12:00:00Z' })).toBe(false);
	});
	it('is not stale for a member whose newest match is constructed', () => {
		// last_bg_played_at is the last BATTLEGROUNDS match, not the last
		// match of any mode, so it equals rating_at even though the member
		// played a (unrated) constructed game more recently.
		expect(hsRatingStale({ rating_at: '2026-09-21T12:00:00Z', last_bg_played_at: '2026-09-21T12:00:00Z' })).toBe(false);
	});
});

describe('hsByRating', () => {
	it('sorts by rating, highest first, members without one last', () => {
		const out = hsByRating([
			player({ username: 'a' }),
			player({ username: 'b', rating: 5000 }),
			player({ username: 'c', rating: 7000 })
		]);
		expect(out.map((p) => p.username)).toEqual(['c', 'b', 'a']);
	});
});

describe('hsRatingSeries', () => {
	it('keeps only rated matches, oldest first, at most twenty', () => {
		const recent: HsRecentMatch[] = [
			{ played_at: '3', mode: 'battlegrounds', result: 'loss', rating_after: 5644 },
			{ played_at: '2', mode: 'battlegrounds', result: 'loss' },
			{ played_at: '1', mode: 'battlegrounds', result: 'loss', rating_after: 5571 }
		];
		expect(hsRatingSeries(recent)).toEqual([5571, 5644]);
		const many = Array.from({ length: 30 }, (_, i) => ({
			played_at: String(30 - i), mode: 'battlegrounds', result: 'loss', rating_after: 6000 - i
		}));
		expect(hsRatingSeries(many)).toHaveLength(20);
		expect(hsRatingSeries(many)[19]).toBe(6000);
	});
});

describe('hsRatingDelta', () => {
	it('is after minus before, or null when either is missing', () => {
		expect(hsRatingDelta({ played_at: '', mode: '', result: '', rating: 5571, rating_after: 5644 })).toBe(73);
		expect(hsRatingDelta({ played_at: '', mode: '', result: '', rating: 5644, rating_after: 5603 })).toBe(-41);
		expect(hsRatingDelta({ played_at: '', mode: '', result: '', rating_after: 5644 })).toBeNull();
	});
});
