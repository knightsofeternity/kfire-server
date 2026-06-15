// REST + WebSocket client for the KFIRE server.
// Contract: https://github.com/knightsofeternity/kfire-protocol

import { get } from 'svelte/store';
import { auth } from './stores/auth.svelte';

export type User = {
	id: string;
	username: string;
	email?: string;
	role: 'admin' | 'member';
	avatar_url?: string;
	activity_visible?: boolean;
	sessions_visible?: boolean;
	created_at: string;
};

export type Game = { id: string; name: string; slug: string; icon_url?: string; cover_url?: string };

export type LeaderboardEntry = {
	user_id: string;
	username: string;
	avatar_url?: string;
	total_seconds: number;
	session_count: number;
};

export type GameAchievement = {
	api_name: string;
	display_name?: string;
	icon_url?: string;
	unlocks: number;
};

export type WowCharacter = {
	user_id: string;
	name: string;
	realm?: string;
	class?: string;
	race?: string;
	faction?: string;
	level?: number;
	item_level: number;
	mythic_rating?: number;
	achievement_points?: number;
};

export type GameDetail = {
	game: Game;
	total_seconds: number;
	player_count: number;
	leaderboard: LeaderboardEntry[];
	achievements: GameAchievement[];
	wow_characters?: WowCharacter[];
	wow_synced_at?: string;
	bnet_profiles?: { user_id: string; username: string; data: Record<string, unknown> }[];
	bnet_synced_at?: string;
};

export type PlayerGameAchievement = {
	api_name: string;
	display_name?: string;
	icon_url?: string;
	unlocked_at: string;
};

export type PlayerWowCharacter = {
	name: string;
	realm?: string;
	realm_slug?: string;
	class?: string;
	race?: string;
	faction?: string;
	level?: number;
	item_level: number;
	mythic_rating?: number;
	achievement_points?: number;
};

export type WowAchievementEntry = {
	id: number;
	name: string;
	completed_at: number; // epoch ms
};

export type PlayerGameDetail = {
	game: Game;
	total_seconds?: number;
	session_count?: number;
	last_played_at?: string;
	wow_characters?: PlayerWowCharacter[];
	bnet_profile?: Record<string, unknown>;
	achievements?: PlayerGameAchievement[];
};

export type AchievementGameOption = { game: Game; count: number };

export type UserAchievements = {
	achievements: Achievement[];
	games: AchievementGameOption[];
	has_more: boolean;
};

export type PlayedGame = {
	id: string;
	name: string;
	slug: string;
	icon_url?: string;
	player_count: number;
	total_seconds: number;
};

export type PresenceEntry = {
	user_id: string;
	username: string;
	avatar_url?: string;
	status: 'offline' | 'online' | 'in_game';
	game?: Game | null;
	since?: string;
};

export type GameStat = {
	game: Game;
	total_seconds: number;
	session_count: number;
	last_played_at: string;
};

export type Session = {
	id: string;
	user_id: string;
	game: Game;
	source: string;
	started_at: string;
	ended_at: string | null;
	duration_seconds?: number;
};

export type Connection = {
	provider: string;
	provider_user_id: string;
	display_name?: string;
	avatar_url?: string;
	profile_url?: string;
	linked_at: string;
	scopes?: string[];
};

export type Achievement = {
	game: Game;
	api_name: string;
	display_name?: string;
	icon_url?: string;
	unlocked_at: string;
};

export type Member = {
	id: string;
	username: string;
	email: string;
	role: 'admin' | 'member';
	banned: boolean;
	avatar_url?: string;
	created_at: string;
};

export type Invite = {
	code: string;
	role: 'admin' | 'member';
	url: string;
	note?: string;
	created_at: string;
	expires_at: string;
};

export type Profile = {
	user: User;
	presence: PresenceEntry;
	total_seconds: number;
	game_stats: GameStat[];
	connections: Connection[];
	achievement_count: number;
	recent_achievements: Achievement[];
};

class ApiError extends Error {
	constructor(
		public code: string,
		message: string
	) {
		super(message);
	}
}

/** Fetch with the access token, refreshing once on 401. */
async function authFetch(path: string, init: RequestInit = {}): Promise<Response> {
	const token = get(auth).accessToken;
	const headers = new Headers(init.headers);
	if (token) headers.set('Authorization', `Bearer ${token}`);
	// Let the browser set the multipart boundary for FormData uploads.
	if (init.body && !(init.body instanceof FormData)) headers.set('Content-Type', 'application/json');

	let res = await fetch(path, { ...init, headers });
	if (res.status === 401 && (await auth.refresh())) {
		headers.set('Authorization', `Bearer ${get(auth).accessToken}`);
		res = await fetch(path, { ...init, headers });
	}
	return res;
}

async function json<T>(res: Response): Promise<T> {
	if (!res.ok) {
		const body = await res.json().catch(() => ({ code: 'unknown', message: res.statusText }));
		throw new ApiError(body.code ?? 'unknown', body.message ?? 'request failed');
	}
	return res.json();
}

export const api = {
	async getMe(): Promise<User> {
		return json(await authFetch('/api/v1/users/me'));
	},

	async updateActivityVisible(visible: boolean): Promise<User> {
		return json(
			await authFetch('/api/v1/users/me', {
				method: 'PATCH',
				body: JSON.stringify({ activity_visible: visible })
			})
		);
	},

	async updateSessionsVisible(visible: boolean): Promise<User> {
		return json(
			await authFetch('/api/v1/users/me', {
				method: 'PATCH',
				body: JSON.stringify({ sessions_visible: visible })
			})
		);
	},

	async getPresence(): Promise<PresenceEntry[]> {
		const data = await json<{ entries: PresenceEntry[] }>(await authFetch('/api/v1/presence'));
		return data.entries;
	},

	async getProfile(id: string): Promise<Profile> {
		return json(await authFetch(`/api/v1/users/${id}`));
	},

	async userGames(id: string): Promise<{ games: { game: Game; source: string }[] }> {
		return json(await authFetch(`/api/v1/users/${encodeURIComponent(id)}/games`));
	},

	async userGameDetail(id: string, slug: string): Promise<PlayerGameDetail> {
		return json(
			await authFetch(`/api/v1/users/${encodeURIComponent(id)}/games/${encodeURIComponent(slug)}`)
		);
	},

	async wowAchievements(
		id: string,
		realm: string,
		name: string
	): Promise<{ achievements: WowAchievementEntry[] }> {
		return json(
			await authFetch(
				`/api/v1/users/${encodeURIComponent(id)}/wow/${encodeURIComponent(realm)}/${encodeURIComponent(name)}/achievements`
			)
		);
	},

	async getGame(slug: string): Promise<GameDetail> {
		return json(await authFetch(`/api/v1/games/${encodeURIComponent(slug)}`));
	},

	async getPlayedGames(): Promise<PlayedGame[]> {
		const data = await json<{ games: PlayedGame[] }>(await authFetch('/api/v1/games/played'));
		return data.games;
	},

	async getUserAchievements(
		userId: string,
		opts: { gameId?: string; offset?: number; limit?: number } = {}
	): Promise<UserAchievements> {
		const p = new URLSearchParams({ user_id: userId });
		if (opts.gameId) p.set('game_id', opts.gameId);
		if (opts.offset) p.set('offset', String(opts.offset));
		if (opts.limit) p.set('limit', String(opts.limit));
		return json(await authFetch(`/api/v1/achievements?${p.toString()}`));
	},

	async resetMemberPassword(id: string): Promise<{ reset_url: string; expires_at: string }> {
		return json(
			await authFetch(`/api/v1/admin/members/${encodeURIComponent(id)}/reset`, { method: 'POST' })
		);
	},

	async getPairInfo(code: string): Promise<{ device_name: string; platform: string }> {
		return json(await authFetch(`/api/v1/devices/pair/${encodeURIComponent(code)}`));
	},

	async approvePair(code: string): Promise<void> {
		const res = await authFetch(`/api/v1/devices/pair/${encodeURIComponent(code)}/approve`, {
			method: 'POST'
		});
		if (!res.ok) {
			const b = await res.json().catch(() => ({ message: 'approval failed' }));
			throw new Error(b.message ?? 'approval failed');
		}
	},

	async getSessions(userId: string, cursor?: string): Promise<{ sessions: Session[]; next_cursor?: string }> {
		const q = new URLSearchParams({ user_id: userId, limit: '20' });
		if (cursor) q.set('cursor', cursor);
		return json(await authFetch(`/api/v1/sessions?${q}`));
	},

	/** Returns the Steam "Sign in through Steam" URL to navigate to. */
	async startSteamLink(): Promise<string> {
		const data = await json<{ url: string }>(await authFetch('/api/v1/connect/steam'));
		return data.url;
	},

	async unlinkSteam(): Promise<void> {
		const res = await authFetch('/api/v1/connect/steam', { method: 'DELETE' });
		if (!res.ok && res.status !== 404) throw new Error('failed to unlink');
	},

	async syncSteam(): Promise<{ games_imported: number; achievements_imported: number }> {
		return json(await authFetch('/api/v1/connect/steam/sync', { method: 'POST' }));
	},

	/** Returns the Battle.net OAuth2 authorization URL to navigate to. */
	async startBattlenetLink(): Promise<string> {
		const data = await json<{ url: string }>(await authFetch('/api/v1/connect/battlenet'));
		return data.url;
	},

	async unlinkBattlenet(): Promise<void> {
		const res = await authFetch('/api/v1/connect/battlenet', { method: 'DELETE' });
		if (!res.ok && res.status !== 404) throw new Error('failed to unlink');
	},

	/** Returns the OpenXBL "Sign in with Xbox" URL to navigate to. */
	async startXboxLink(): Promise<string> {
		const data = await json<{ url: string }>(await authFetch('/api/v1/connect/xbox'));
		return data.url;
	},

	async unlinkXbox(): Promise<void> {
		const res = await authFetch('/api/v1/connect/xbox', { method: 'DELETE' });
		if (!res.ok && res.status !== 404) throw new Error('failed to unlink');
	},

	// --- admin ---------------------------------------------------------------

	async getMembers(): Promise<Member[]> {
		const data = await json<{ members: Member[] }>(await authFetch('/api/v1/admin/members'));
		return data.members;
	},

	async patchMember(id: string, body: { role?: string; banned?: boolean }): Promise<void> {
		await json(
			await authFetch(`/api/v1/admin/members/${id}`, { method: 'PATCH', body: JSON.stringify(body) })
		);
	},

	async getInvites(): Promise<Invite[]> {
		const data = await json<{ invites: Invite[] }>(await authFetch('/api/v1/admin/invites'));
		return data.invites;
	},

	async createInvite(body: { note?: string; role?: string }): Promise<Invite> {
		return json(await authFetch('/api/v1/admin/invites', { method: 'POST', body: JSON.stringify(body) }));
	},

	async deleteInvite(code: string): Promise<void> {
		const res = await authFetch(`/api/v1/admin/invites/${encodeURIComponent(code)}`, {
			method: 'DELETE'
		});
		if (!res.ok && res.status !== 404) throw new Error('failed to revoke invite');
	},

	async getBranding(): Promise<{ accent: string; has_logo: boolean }> {
		return json(await authFetch('/api/v1/admin/branding'));
	},

	async setAccent(accent: string): Promise<void> {
		await json(await authFetch('/api/v1/admin/branding', {
			method: 'PATCH',
			body: JSON.stringify({ accent })
		}));
	},

	async uploadLogo(file: File): Promise<void> {
		const form = new FormData();
		form.append('logo', file);
		const res = await authFetch('/api/v1/admin/branding/logo', { method: 'POST', body: form });
		if (!res.ok) {
			const body = await res.json().catch(() => ({ message: 'upload failed' }));
			throw new ApiError(body.code ?? 'unknown', body.message ?? 'upload failed');
		}
	},

	async deleteLogo(): Promise<void> {
		const res = await authFetch('/api/v1/admin/branding/logo', { method: 'DELETE' });
		if (!res.ok && res.status !== 404) throw new Error('failed to remove logo');
	},

	async listApiKeys(): Promise<{
		keys: Array<{
			id: string;
			label: string;
			key_prefix: string;
			created_at: string;
			last_used_at?: string;
			revoked: boolean;
		}>;
	}> {
		return json(await authFetch('/api/v1/admin/api-keys'));
	},

	async createApiKey(label: string): Promise<{ id: string; label: string; key_prefix: string; key: string }> {
		return json(
			await authFetch('/api/v1/admin/api-keys', {
				method: 'POST',
				body: JSON.stringify({ label })
			})
		);
	},

	async revokeApiKey(id: string): Promise<void> {
		const res = await authFetch(`/api/v1/admin/api-keys/${id}`, { method: 'DELETE' });
		if (!res.ok && res.status !== 404) throw new Error('failed to revoke API key');
	}
};

/** Public instance config (no auth) - drives the sign-up UI and branding. */
export async function getConfig(): Promise<{
	open_registration: boolean;
	org_name: string;
	needs_setup: boolean;
	accent: string;
	has_logo: boolean;
	connectors: { steam: boolean; battlenet: boolean; xbox: boolean };
}> {
	const res = await fetch('/api/v1/config');
	return res.ok
		? res.json()
		: {
				open_registration: true,
				org_name: 'KFIRE',
				needs_setup: false,
				accent: 'orange',
				has_logo: false,
				// Fail open: if config can't be loaded, still offer the connectors
				// rather than hiding working ones on a transient error.
				connectors: { steam: true, battlenet: true, xbox: true }
			};
}

/** Validate an admin-issued password reset link (no auth). */
export async function peekReset(token: string): Promise<{ username: string }> {
	const res = await fetch(`/api/v1/auth/reset/${encodeURIComponent(token)}`);
	if (!res.ok) throw new ApiError('invalid_token', 'this reset link is invalid or expired');
	return res.json();
}

/** Set a new password from a reset link (no auth). */
export async function submitReset(token: string, password: string): Promise<void> {
	const res = await fetch(`/api/v1/auth/reset/${encodeURIComponent(token)}`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ password })
	});
	if (!res.ok) {
		const body = await res.json().catch(() => ({ code: 'unknown', message: 'reset failed' }));
		throw new ApiError(body.code ?? 'unknown', body.message ?? 'reset failed');
	}
}

export { ApiError };
