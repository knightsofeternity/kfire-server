// WoW Forever guild page helpers (KFireGuilde addon, lot 5).

import type { WowGuildCrafter, WowGuildProfession, WowGuildRosterEntry } from './api';

/**
 * The addon sends the in-game class token (MAGE, DEATHKNIGHT...); the shared
 * colour and icon helpers in wow.ts take Blizzard's display name.
 */
const CLASS_TOKENS: Record<string, string> = {
	DEATHKNIGHT: 'Death Knight',
	DEMONHUNTER: 'Demon Hunter',
	DRUID: 'Druid',
	EVOKER: 'Evoker',
	HUNTER: 'Hunter',
	MAGE: 'Mage',
	MONK: 'Monk',
	PALADIN: 'Paladin',
	PRIEST: 'Priest',
	ROGUE: 'Rogue',
	SHAMAN: 'Shaman',
	WARLOCK: 'Warlock',
	WARRIOR: 'Warrior'
};

/** Display name of a class token; an unknown token is returned as is. */
export function wowClassName(token?: string): string | undefined {
	if (!token) return undefined;
	return CLASS_TOKENS[token.toUpperCase().replace(/[\s_]/g, '')] ?? token;
}

/** "Ouranos-Chogall" -> "Ouranos". Names never hold a hyphen; realms may. */
export function shortName(full?: string): string {
	if (!full) return '';
	const i = full.indexOf('-');
	return i > 0 ? full.slice(0, i) : full;
}

export type RosterSort = 'rank' | 'level' | 'name' | 'class' | 'seen' | 'pseudo';

/**
 * Sorts a copy of the roster. Missing values always go last, whatever the
 * direction, and the name breaks ties so the order never wobbles.
 */
export function sortRoster(
	rows: WowGuildRosterEntry[],
	by: RosterSort,
	desc = false
): WowGuildRosterEntry[] {
	const key = (r: WowGuildRosterEntry): number | string | undefined => {
		switch (by) {
			case 'rank':
				return r.rank_index;
			case 'level':
				return r.level;
			case 'class':
				return wowClassName(r.class);
			case 'seen':
				return r.last_seen_at ? new Date(r.last_seen_at).getTime() : undefined;
			case 'pseudo':
				return r.pseudo?.toLowerCase();
			default:
				return r.name.toLowerCase();
		}
	};
	return [...rows].sort((a, b) => {
		const ka = key(a);
		const kb = key(b);
		if (ka === undefined && kb !== undefined) return 1;
		if (kb === undefined && ka !== undefined) return -1;
		let c = 0;
		if (ka !== undefined && kb !== undefined) {
			c = typeof ka === 'number' && typeof kb === 'number' ? ka - kb : String(ka).localeCompare(String(kb));
			if (desc) c = -c;
		}
		return c || a.name.localeCompare(b.name);
	});
}

export type CraftHit = {
	profession: string;
	id: string;
	name?: string;
	crafters: WowGuildCrafter[];
};

/** Lowercase, accents stripped, so "etoffe" finds "étoffe". */
function fold(s: string): string {
	return s
		.normalize('NFD')
		.replace(/\p{Diacritic}/gu, '')
		.toLowerCase();
}

/**
 * "Who can craft X": every recipe whose name (or id) contains the query, with
 * the characters who know it. At most `limit` hits, named recipes first.
 */
export function whoCrafts(professions: WowGuildProfession[], query: string, limit = 50): CraftHit[] {
	const q = fold(query.trim());
	if (q.length < 2) return [];
	const hits: CraftHit[] = [];
	for (const p of professions) {
		const byName = new Map(p.crafters.map((c) => [c.name, c]));
		for (const r of p.recipes) {
			if (!fold(r.name ?? '').includes(q) && r.id !== q) continue;
			hits.push({
				profession: p.profession,
				id: r.id,
				name: r.name,
				crafters: r.crafters.map((n) => byName.get(n) ?? { name: n, updated_at: '' })
			});
		}
	}
	return hits
		.sort((a, b) => (a.name ?? '￿').localeCompare(b.name ?? '￿') || a.id.localeCompare(b.id))
		.slice(0, limit);
}

/** Wowhead link for a recipe id: items by number, spells by "s" + number. */
export function wowheadURL(id: string): string {
	return id.startsWith('s')
		? `https://www.wowhead.com/classic/spell=${id.slice(1)}`
		: `https://www.wowhead.com/classic/item=${id}`;
}

/** Heat for a presence cell, 0..1 against the week's busiest hour. */
export function heat(value: number | null | undefined, max: number): number {
	if (value == null || max <= 0) return 0;
	return Math.min(1, value / max);
}
