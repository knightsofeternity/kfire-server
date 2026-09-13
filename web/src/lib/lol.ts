import type { LolPlayer, LolProfile, LolRank } from './api';

export const SOLO_QUEUE = 'RANKED_SOLO_5x5';
export const FLEX_QUEUE = 'RANKED_FLEX_SR';

/** The ten ranked tiers, lowest first. Keys match the vendored crest files. */
export const TIERS = [
	'iron', 'bronze', 'silver', 'gold', 'platinum',
	'emerald', 'diamond', 'master', 'grandmaster', 'challenger'
] as const;
export type Tier = (typeof TIERS)[number];

const TIER_SET = new Set<string>(TIERS);

/** Riot answers in upper case; our asset names are lower case. */
export function tierKey(tier: string): Tier | null {
	const k = tier.toLowerCase();
	return TIER_SET.has(k) ? (k as Tier) : null;
}

/**
 * Path to a tier crest, or null for a tier we do not know. Riot could add one,
 * and a missing crest must leave the row intact rather than break the page.
 */
export function crestURL(tier: string): string | null {
	const k = tierKey(tier);
	return k ? `/lol-tiers/${k}.svg` : null;
}

/**
 * Loading-screen art for a champion, 40 kB in portrait format. Splash art was
 * considered and dropped: three times heavier and it crops badly into a card.
 * Returns null when the image id is missing, so the card keeps a flat ground.
 */
export function loadingArtURL(imageID: string | undefined): string | null {
	return imageID
		? `https://ddragon.leagueoflegends.com/cdn/img/champion/loading/${imageID}_0.jpg`
		: null;
}

export function soloRank(p: { data: LolProfile }): LolRank | undefined {
	return p.data.ranks.find((r) => r.queue === SOLO_QUEUE);
}

export function flexRank(p: { data: LolProfile }): LolRank | undefined {
	return p.data.ranks.find((r) => r.queue === FLEX_QUEUE);
}

export function winRate(r: { wins: number; losses: number }): number {
	const total = r.wins + r.losses;
	return total > 0 ? Math.round((r.wins * 100) / total) : 0;
}

/** The member's highest-mastery champion, used for the podium art. */
export function mainChampion(p: { data: LolProfile }) {
	return p.data.top_champions[0];
}

/**
 * The champion appearing most often across every member's last five matches.
 * Ties break on the first one seen, which keeps the answer stable between two
 * renders of the same data.
 */
export function mostPlayedChampion(players: LolPlayer[]): string | null {
	const counts = new Map<string, number>();
	for (const p of players) {
		for (const m of p.data.recent) {
			if (!m.champion) continue;
			counts.set(m.champion, (counts.get(m.champion) ?? 0) + 1);
		}
	}
	let best: string | null = null;
	let bestN = 0;
	for (const [champ, n] of counts) {
		if (n > bestN) {
			best = champ;
			bestN = n;
		}
	}
	return best;
}

/** Members in a game right now. The server already filtered out those who hid their activity. */
export function inGame(players: LolPlayer[]): LolPlayer[] {
	return players.filter((p) => p.live);
}

/**
 * The top three, for the podium. Empty unless at least three members are ranked
 * in solo queue: a three-place podium with two ranked members makes no sense.
 */
export function podium(players: LolPlayer[]): LolPlayer[] {
	const ranked = players.filter((p) => soloRank(p));
	return ranked.length >= 3 ? ranked.slice(0, 3) : [];
}

/**
 * Each tier's own colour, close to Riot's crests, so the spread bar reads as a
 * ladder rather than one flat block. Applied inline: Tailwind cannot build a
 * class name from a runtime value.
 */
const TIER_COLOURS: Record<Tier | 'unranked', string> = {
	iron: '#7d7a78',
	bronze: '#a06a3c',
	silver: '#9aa6b1',
	gold: '#e0b23c',
	platinum: '#4fc3b0',
	emerald: '#2fb673',
	diamond: '#5aa6f2',
	master: '#b05cf0',
	grandmaster: '#e0453b',
	challenger: '#f2cd5c',
	unranked: '#5c5c66'
};

export function tierColour(tier: Tier | 'unranked'): string {
	return TIER_COLOURS[tier];
}

export type TierSlice = { tier: Tier | 'unranked'; count: number };

/**
 * Tier spread across the guild, ordered from the highest tier down, unranked
 * last. Only tiers actually present are returned, so the bar has no empty
 * segments.
 */
export function tierSpread(players: LolPlayer[]): TierSlice[] {
	const counts = new Map<string, number>();
	for (const p of players) {
		const r = soloRank(p);
		const k = r ? (tierKey(r.tier) ?? 'unranked') : 'unranked';
		counts.set(k, (counts.get(k) ?? 0) + 1);
	}
	const ordered: (Tier | 'unranked')[] = [...TIERS].reverse();
	ordered.push('unranked');
	return ordered
		.filter((k) => counts.has(k))
		.map((k) => ({ tier: k, count: counts.get(k) as number }));
}
