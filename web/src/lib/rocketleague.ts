import type { RlPlayer, RlMatch } from './api';

/**
 * Win rate across every reported match, as a whole percentage.
 * Zero matches yields 0 rather than a division by zero.
 */
export function rlWinRate(p: RlPlayer): number {
	return p.matches > 0 ? Math.round((p.wins * 100) / p.matches) : 0;
}

/**
 * Members ranked by wins, then win rate, then matches played.
 *
 * Wins and not win rate: a member who has played one match and won it sits at
 * 100%, and ranking on that would put him above someone at 65% over forty
 * matches. An absolute count cannot be inflated by playing less, and it reads
 * the same way on the first evening as on the hundredth.
 */
export function rlByWins(players: RlPlayer[]): RlPlayer[] {
	return [...players].sort((a, b) => {
		if (a.wins !== b.wins) return b.wins - a.wins;
		const wa = rlWinRate(a);
		const wb = rlWinRate(b);
		if (wa !== wb) return wb - wa;
		return b.matches - a.matches || a.username.localeCompare(b.username);
	});
}

/** Total matches reported by the guild. */
export function rlTotalMatches(players: RlPlayer[]): number {
	return players.reduce((n, p) => n + p.matches, 0);
}

/** Total goals scored by the guild across every reported match. */
export function rlTotalGoals(players: RlPlayer[]): number {
	return players.reduce((n, p) => n + p.goals, 0);
}

/**
 * The mode label for a match, derived from `team_size` rather than from
 * `playlist`.
 *
 * Rocket League's protocol never sends the playlist, so `playlist` is almost
 * always null; even on the rare day it is present, team size is still the
 * useful fact and a raw playlist number would mean nothing to a member (it
 * is an internal Psyonix identifier). So the mode shown here always comes
 * from team size, and `playlist` is not read at all.
 */
export function rlModeLabel(match: Pick<RlMatch, 'team_size'>): string {
	return `${match.team_size}v${match.team_size}`;
}

/** The member's own score line, blue-vs-orange, with his side identified. */
export function rlSideScore(m: RlMatch): { own: number; opponent: number } {
	return m.player_team === 0
		? { own: m.team_blue_score, opponent: m.team_orange_score }
		: { own: m.team_orange_score, opponent: m.team_blue_score };
}
