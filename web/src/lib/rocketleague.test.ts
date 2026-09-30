import { describe, expect, it } from 'vitest';
import { rlModeLabel } from './rocketleague';

describe('rlModeLabel', () => {
	it('is the team size on a standard arena', () => {
		expect(rlModeLabel({ team_size: 2, mode: null })).toBe('2v2');
		expect(rlModeLabel({ team_size: 3 })).toBe('3v3');
	});
	it('names the mode when the arena gave it away', () => {
		expect(rlModeLabel({ team_size: 2, mode: 'hoops' })).toBe('2v2 · Hoops');
		expect(rlModeLabel({ team_size: 3, mode: 'dropshot' })).toBe('3v3 · Dropshot');
	});
});
