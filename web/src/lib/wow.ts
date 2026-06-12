// WoW class utilities shared between game and player-game pages.

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
