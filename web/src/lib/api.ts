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
	created_at: string;
};

export type Game = { id: string; name: string; slug: string; icon_url?: string };

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
	if (init.body) headers.set('Content-Type', 'application/json');

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

	async getPresence(): Promise<PresenceEntry[]> {
		const data = await json<{ entries: PresenceEntry[] }>(await authFetch('/api/v1/presence'));
		return data.entries;
	},

	async getProfile(id: string): Promise<Profile> {
		return json(await authFetch(`/api/v1/users/${id}`));
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
	}
};

/** Public instance config (no auth) — drives the sign-up UI. */
export async function getConfig(): Promise<{ open_registration: boolean; org_name: string }> {
	const res = await fetch('/api/v1/config');
	return res.ok ? res.json() : { open_registration: true, org_name: 'KFIRE' };
}

export { ApiError };
