// WoW class utilities shared between game and player-game pages.

import type { WowCharacter } from './api';

export const WOW_CLASS_COLORS: Record<string, string> = {
	'Death Knight': '#C41E3A',
	'Demon Hunter': '#A330C9',
	Druid: '#FF7C0A',
	Evoker: '#33937F',
	Hunter: '#AAD372',
	Mage: '#3FC7EB',
	Monk: '#00FF98',
	Paladin: '#F48CBA',
	Priest: '#FFFFFF',
	Rogue: '#FFF468',
	Shaman: '#0070DD',
	Warlock: '#8788EE',
	Warrior: '#C69B6D'
};

/** Returns the WoW class color, or a CSS variable fallback if unknown. */
export function wowClassColor(cls?: string): string {
	if (!cls) return 'var(--color-muted)';
	// Case-insensitive lookup
	const key = Object.keys(WOW_CLASS_COLORS).find(
		(k) => k.toLowerCase() === cls.toLowerCase()
	);
	return key ? WOW_CLASS_COLORS[key] : 'var(--color-muted)';
}

/** Returns the Wowhead class icon URL, or undefined if no class provided. */
export function wowClassIcon(cls?: string): string | undefined {
	if (!cls) return undefined;
	const slug = cls.toLowerCase().replace(/\s+/g, '');
	return `https://wow.zamimg.com/images/wow/icons/large/classicon_${slug}.jpg`;
}

/**
 * Game versions, in display order. The keys match what the server sends;
 * "unknown" is the local bucket for characters written before the version
 * column existed.
 */
export const WOW_VERSIONS = ['retail', 'classic-progression', 'classic-era', 'unknown'] as const;
export type WowVersion = (typeof WOW_VERSIONS)[number];

/**
 * Display names. These are proper nouns, identical in every language, so they
 * live here rather than in the i18n catalogues.
 *
 * MAINTENANCE: "Mists of Pandaria Classic" names the Classic progression
 * realms, which Blizzard advances roughly once a year. Update this one line
 * when they move to the next expansion. The server never sends this name, only
 * the key, so nothing else changes.
 */
const VERSION_NAMES: Record<WowVersion, string> = {
	retail: 'Retail',
	'classic-progression': 'Mists of Pandaria Classic',
	'classic-era': 'Classic Era',
	unknown: ''
};

/**
 * Wowhead icons, one per version. Blizzard's own expansion logos answer 403 and
 * hosting them ourselves raises a licensing question, so these square item
 * icons are the accepted compromise. Each was checked to answer 200.
 */
const VERSION_ICONS: Record<WowVersion, string> = {
	retail: 'inv_misc_head_dragon_black',
	'classic-progression': 'achievement_zone_valeofeternalblossoms',
	'classic-era': 'achievement_zone_easternkingdoms_01',
	unknown: ''
};

export function wowVersionName(v: WowVersion): string {
	return VERSION_NAMES[v];
}

/** Icon URL for a version, or undefined for the neutral bucket. */
export function wowVersionIcon(v: WowVersion): string | undefined {
	const icon = VERSION_ICONS[v];
	return icon ? `https://wow.zamimg.com/images/wow/icons/large/${icon}.jpg` : undefined;
}

/** Normalises the server's value; anything unrecognised falls in the neutral bucket. */
export function wowVersionKey(v: string | undefined): WowVersion {
	return v && (WOW_VERSIONS as readonly string[]).includes(v) && v !== 'unknown'
		? (v as WowVersion)
		: 'unknown';
}

export type WowVersionGroup = { version: WowVersion; characters: WowCharacter[] };
export type WowRoster = {
	userId: string;
	username: string;
	avatarUrl?: string;
	total: number;
	groups: WowVersionGroup[];
};

/**
 * Groups every character by member, then by version.
 *
 * Members are ordered by character count then name. That is NOT a ranking:
 * no number is comparable across versions, so the page deliberately ranks
 * nobody. It is only a stable, deterministic order.
 *
 * Within a version, characters come out highest level first, then highest item
 * level, then by name so the order never wobbles between two renders.
 */
export function wowRosters(characters: WowCharacter[]): WowRoster[] {
	const byMember = new Map<string, WowCharacter[]>();
	for (const ch of characters) {
		const list = byMember.get(ch.user_id);
		if (list) list.push(ch);
		else byMember.set(ch.user_id, [ch]);
	}

	const rosters: WowRoster[] = [];
	for (const [userId, chars] of byMember) {
		const groups: WowVersionGroup[] = [];
		for (const version of WOW_VERSIONS) {
			const inVersion = chars
				.filter((c) => wowVersionKey(c.version) === version)
				.sort(
					(a, b) =>
						(b.level ?? 0) - (a.level ?? 0) ||
						b.item_level - a.item_level ||
						a.name.localeCompare(b.name)
				);
			if (inVersion.length) groups.push({ version, characters: inVersion });
		}
		rosters.push({
			userId,
			username: chars[0].username,
			avatarUrl: chars[0].avatar_url,
			total: chars.length,
			groups
		});
	}

	return rosters.sort((a, b) => b.total - a.total || a.username.localeCompare(b.username));
}

/** How many members own at least one character. */
export function wowMemberCount(characters: WowCharacter[]): number {
	return new Set(characters.map((c) => c.user_id)).size;
}
