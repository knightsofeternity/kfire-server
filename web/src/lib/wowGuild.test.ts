import { describe, expect, it } from 'vitest';
import type { WowGuildProfession, WowGuildRosterEntry } from './api';
import { heat, shortName, sortRoster, whoCrafts, wowClassName, wowheadURL } from './wowGuild';

const roster: WowGuildRosterEntry[] = [
	{ name: 'Ouranos-Chogall', level: 60, class: 'MAGE', rank_index: 1 },
	{ name: 'Malkai-Chogall', level: 40, class: 'DEATHKNIGHT', rank_index: 0 },
	{ name: 'Nolevel-Chogall' }
];

describe('wowGuild', () => {
	it('names class tokens', () => {
		expect(wowClassName('DEATHKNIGHT')).toBe('Death Knight');
		expect(wowClassName('mage')).toBe('Mage');
		expect(wowClassName('ODD')).toBe('ODD');
		expect(wowClassName(undefined)).toBeUndefined();
	});

	it('shortens names', () => {
		expect(shortName('Ab-Azjol-Nerub')).toBe('Ab');
		expect(shortName('Solo')).toBe('Solo');
	});

	it('sorts the roster with missing values last', () => {
		expect(sortRoster(roster, 'level', true).map((r) => r.level)).toEqual([60, 40, undefined]);
		expect(sortRoster(roster, 'level').map((r) => r.level)).toEqual([40, 60, undefined]);
		expect(sortRoster(roster, 'rank')[0].name).toBe('Malkai-Chogall');
	});

	it('finds who crafts a recipe, accents ignored', () => {
		const profs: WowGuildProfession[] = [
			{
				profession: 'Couture',
				crafters: [{ name: 'Malkai-Chogall', level: 75, max: 150, updated_at: '2026-11-20T19:00:00Z' }],
				recipes: [
					{ id: '2996', name: "Rouleau d'étoffe de lin", crafters: ['Malkai-Chogall'] },
					{ id: '4343', crafters: ['Malkai-Chogall'] }
				]
			}
		];
		const hits = whoCrafts(profs, 'etoffe');
		expect(hits).toHaveLength(1);
		expect(hits[0].crafters[0].level).toBe(75);
		expect(whoCrafts(profs, '4343')).toHaveLength(1);
		expect(whoCrafts(profs, 'x')).toEqual([]);
	});

	it('links wowhead and scales heat', () => {
		expect(wowheadURL('s7418')).toContain('spell=7418');
		expect(wowheadURL('2996')).toContain('item=2996');
		expect(heat(5, 10)).toBe(0.5);
		expect(heat(null, 10)).toBe(0);
	});
});
