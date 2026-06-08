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

export type Profile = {
	user: User;
	presence: PresenceEntry;
	total_seconds: number;
	game_stats: GameStat[];
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
	}
};

export { ApiError };
